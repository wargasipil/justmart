package product_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	productsvc "github.com/justmart/backend/internal/service/product"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestListProducts_PaginatesAndFilters(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, svc, ctx, "SKU-LP-1", "Aspirin", 1000)
	seedProduct(t, svc, ctx, "SKU-LP-2", "Ibuprofen", 1100)
	seedProduct(t, svc, ctx, "SKU-LP-3", "Loratadine", 1200)

	// No filter: all three (active) products, total reflects unfiltered count.
	resp, err := svc.ListProducts(ctx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(3), resp.Msg.Total)
	require.Len(t, resp.Msg.Products, 3)

	// Query filter narrows to the matching product.
	resp, err = svc.ListProducts(ctx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{
		Query: "Ibupro",
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.Len(t, resp.Msg.Products, 1)
	require.Equal(t, "Ibuprofen", resp.Msg.Products[0].Name)
}

// TestListProducts_EnrichesLastRestock proves the list fills last_restock_* from
// the latest restock across suppliers in the active warehouse (and leaves them
// zero before any restock).
func TestListProducts_EnrichesLastRestock(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	mainWH := defaultWarehouseID(t, gormDB)
	prodID := seedProduct(t, svc, ctx, "lr-sku", "LR product", 1000)
	supA := seedSupplierRow(t, gormDB, "LR-A", "Supplier A")
	supB := seedSupplierRow(t, gormDB, "LR-B", "Supplier B")

	// Before any restock: fields are zero/empty.
	resp, err := svc.ListProducts(ctx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{Query: "LR product"}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Products, 1)
	require.Equal(t, int64(0), resp.Msg.Products[0].LastRestockPrice)
	require.Empty(t, resp.Msg.Products[0].LastRestockSupplierId)

	// Two suppliers restocked; supB arrived later → it is the "latest".
	seedRestockLast(t, gormDB, mainWH, prodID, supA, 900, 5, date(2026, 1, 3))
	seedRestockLast(t, gormDB, mainWH, prodID, supB, 950, 7, date(2026, 2, 3))

	resp, err = svc.ListProducts(ctx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{Query: "LR product"}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Products, 1)
	p := resp.Msg.Products[0]
	require.Equal(t, int64(950), p.LastRestockPrice) // latest = supB
	require.Equal(t, int64(7), p.LastRestockQty)
	require.Equal(t, supB, p.LastRestockSupplierId)
	require.NotZero(t, p.LastRestockArrivedAt)
}

func TestListProducts_InvalidOpnameBefore(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.ListProducts(ctx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{
		OpnameBefore: "not-a-date",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestListProducts_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := productsvc.NewProductService(gormDB)

	_, err := svc.ListProducts(context.Background(), connect.NewRequest(&inventoryifacev1.ListProductsRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

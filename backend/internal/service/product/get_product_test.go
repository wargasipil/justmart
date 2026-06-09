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

func TestGetProduct_WithStock(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	pid := seedProduct(t, svc, ctx, "SKU-GET-1", "Cetirizine 10mg", 2000)
	whID := defaultWarehouseID(t, gormDB)
	seedBatchWithStock(t, gormDB, pid, whID, ownerID, 40, 1200)

	resp, err := svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: pid}))
	require.NoError(t, err)
	p := resp.Msg.Product
	require.NotNil(t, p)
	require.Equal(t, pid, p.Id)
	require.Equal(t, "Cetirizine 10mg", p.Name)
	require.Equal(t, int64(40), p.ReadyStock)          // active-warehouse on-hand
	require.Equal(t, int64(40), p.TotalStock)          // single warehouse -> equals ready
	require.Equal(t, int64(40*1200), p.StockValuation) // qty * cost_price
	require.Equal(t, int64(1200), p.ReferenceCost)     // latest batch cost
	require.NotEmpty(t, p.LastRestockDate)             // a positive movement exists
	require.NotEmpty(t, p.Units)
}

func TestGetProduct_NotFound(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetProduct_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := productsvc.NewProductService(gormDB)

	_, err := svc.GetProduct(context.Background(), connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: "x"}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

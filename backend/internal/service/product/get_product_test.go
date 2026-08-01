package product_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
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

// On-order carries a valuation alongside the qty, priced at the line's NET
// per-base cost — the same rate CreateReceipt will stamp on the batch — so a
// discounted PO doesn't overstate incoming value.
func TestGetProduct_OnOrderQtyAndValuation(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	pid := seedProduct(t, svc, ctx, "SKU-GET-ONORDER", "Amoxicillin 500mg", 3000)
	whID := defaultWarehouseID(t, gormDB)
	supID := seedSupplierRow(t, gormDB, "SUP-ONORDER", "On-order supplier")

	// Line A: 10 ordered, 4 received -> 6 outstanding. Gross 1000/base, but the
	// line is discounted to a 9000 net -> 900/base.
	seedOpenPOItem(t, gormDB, pid, supID, whID, ownerID, 10, 4, 1000, 9000)
	// Line B: 5 ordered, none received, no discount -> 5 outstanding @ 2000.
	seedOpenPOItem(t, gormDB, pid, supID, whID, ownerID, 5, 0, 2000, 10000)

	resp, err := svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: pid}))
	require.NoError(t, err)
	p := resp.Msg.Product
	require.Equal(t, int64(11), p.OnOrderStock)               // 6 + 5
	require.Equal(t, int64(6*900+5*2000), p.OnOrderValuation) // net rate, not gross
	require.Equal(t, int64(0), p.ReadyStock)                  // nothing received into stock
	require.Equal(t, int64(0), p.StockValuation)
}

// A fully-received or voided PO is not "ongoing" — neither its qty nor its value
// may leak into the tile.
func TestGetProduct_OnOrderExcludesClosedPOs(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	pid := seedProduct(t, svc, ctx, "SKU-GET-CLOSED", "Ibuprofen 400mg", 1500)
	whID := defaultWarehouseID(t, gormDB)
	supID := seedSupplierRow(t, gormDB, "SUP-CLOSED", "Closed PO supplier")

	poID := seedOpenPOItem(t, gormDB, pid, supID, whID, ownerID, 8, 0, 500, 4000)
	require.NoError(t, gormDB.Model(&model.PurchaseOrder{}).
		Where("id = ?", poID).Update("status", "VOIDED").Error)

	resp, err := svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: pid}))
	require.NoError(t, err)
	require.Equal(t, int64(0), resp.Msg.Product.OnOrderStock)
	require.Equal(t, int64(0), resp.Msg.Product.OnOrderValuation)
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

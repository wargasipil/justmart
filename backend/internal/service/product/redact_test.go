package product_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	productsvc "github.com/justmart/backend/internal/service/product"
	"github.com/justmart/backend/internal/service/servicetest"
)

// The catalog is FULLY READABLE by the till: cost included (common.CanSeeCost
// is true for every role). These two tests seed a shop and assert the manager
// and the till get the SAME figures — they are what would fail if redactCost
// started blanking again, and they sit at the handler boundary because that is
// where the redactor runs (last step, after every enrich).
//
// The write side is unaffected: Create/Update/Archive/Import remain
// OWNER+PHARMACIST via their proto allowed_roles, enforced by the interceptor
// rather than here.

func TestListProducts_TillSeesCost(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ownerCtx := servicetest.OwnerCtx(context.Background(), ownerID)

	whID := defaultWarehouseID(t, gormDB)
	pid := seedProduct(t, svc, ownerCtx, "SKU-REDACT-1", "Redact product", 2000)
	supID := seedSupplierRow(t, gormDB, "SUP-REDACT", "Redact supplier")
	seedRestockLast(t, gormDB, whID, pid, supID, 1200, 30, time.Now().Add(-24*time.Hour))
	seedOpenPOItem(t, gormDB, pid, supID, whID, ownerID, 10, 0, 1000, 10000)

	// Manager first: proves the enrich actually populated something, so the till
	// assertions below are comparing against real data rather than an empty
	// fixture that would pass either way.
	resp, err := svc.ListProducts(ownerCtx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{Query: "Redact product"}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Products, 1)
	mgr := resp.Msg.Products[0]
	require.Equal(t, int64(1200), mgr.LastRestockPrice)
	require.Equal(t, supID, mgr.LastRestockSupplierId)
	require.Equal(t, int64(10*1000), mgr.OnOrderValuation)

	for _, role := range []string{"CASHIER", "APOTEKER"} {
		tillCtx := servicetest.CtxAs(context.Background(), role, ownerID)
		resp, err := svc.ListProducts(tillCtx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{Query: "Redact product"}))
		require.NoError(t, err, role)
		require.Len(t, resp.Msg.Products, 1, role)
		p := resp.Msg.Products[0]

		// The whole restock block, field for field — this is the list the
		// redactor used to blank, so it is the list that has to survive.
		require.Equal(t, mgr.LastRestockPrice, p.LastRestockPrice, role)
		require.Equal(t, mgr.LastRestockQty, p.LastRestockQty, role)
		require.Equal(t, mgr.LastRestockDiscountType, p.LastRestockDiscountType, role)
		require.Equal(t, mgr.LastRestockDiscountValue, p.LastRestockDiscountValue, role)
		require.Equal(t, mgr.LastRestockCreatedAt, p.LastRestockCreatedAt, role)
		require.Equal(t, mgr.LastRestockArrivedAt, p.LastRestockArrivedAt, role)
		require.Equal(t, mgr.LastRestockSupplierId, p.LastRestockSupplierId, role)
		require.Equal(t, mgr.OnOrderValuation, p.OnOrderValuation, role)

		// Sell-side data and quantities are unchanged by any of this.
		require.Equal(t, int64(2000), p.UnitPrice, role)
		require.Equal(t, int64(10), p.OnOrderStock, role)
		require.NotEmpty(t, p.Units, role)
	}
}

func TestGetProduct_TillSeesCost(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ownerCtx := servicetest.OwnerCtx(context.Background(), ownerID)

	whID := defaultWarehouseID(t, gormDB)
	pid := seedProduct(t, svc, ownerCtx, "SKU-REDACT-2", "Redact detail", 2000)
	seedBatchWithStock(t, gormDB, pid, whID, ownerID, 40, 1200)

	resp, err := svc.GetProduct(ownerCtx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: pid}))
	require.NoError(t, err)
	mgr := resp.Msg.Product
	require.Equal(t, int64(40*1200), mgr.StockValuation)
	require.Equal(t, int64(1200), mgr.ReferenceCost)

	for _, role := range []string{"CASHIER", "APOTEKER"} {
		tillCtx := servicetest.CtxAs(context.Background(), role, ownerID)
		resp, err := svc.GetProduct(tillCtx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: pid}))
		require.NoError(t, err, role)
		p := resp.Msg.Product

		require.Equal(t, mgr.StockValuation, p.StockValuation, role)
		require.Equal(t, mgr.ReferenceCost, p.ReferenceCost, role)
		require.Equal(t, mgr.LastRestockDate, p.LastRestockDate, role)
		require.Equal(t, mgr.LastRestockSupplier, p.LastRestockSupplier, role)

		require.Equal(t, int64(40), p.ReadyStock, role)
		require.Equal(t, int64(40), p.TotalStock, role)
		require.Equal(t, int64(2000), p.UnitPrice, role)
	}
}

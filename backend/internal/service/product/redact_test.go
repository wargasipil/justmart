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

// The till roles read the catalog (POS + the read-only Products pages), so cost
// must be stripped from the RESPONSE, not just hidden in the UI. Both tests seed
// the same shop and assert the manager sees every cost figure while the till
// sees zeros — a single missing line in redactCost fails one of them.

func TestListProducts_RedactsCostForTillRoles(t *testing.T) {
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

	// Manager: the enrich actually populated something, so the till assertions
	// below are proving redaction rather than an empty fixture.
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

		require.Zero(t, p.LastRestockPrice, role)
		require.Zero(t, p.LastRestockQty, role)
		require.Empty(t, p.LastRestockDiscountType, role)
		require.Zero(t, p.LastRestockDiscountValue, role)
		require.Zero(t, p.LastRestockCreatedAt, role)
		require.Zero(t, p.LastRestockArrivedAt, role)
		require.Empty(t, p.LastRestockSupplierId, role)
		require.Zero(t, p.OnOrderValuation, role)

		// Sell-side data and quantities still come through — a cashier prices
		// and counts with these.
		require.Equal(t, int64(2000), p.UnitPrice, role)
		require.Equal(t, int64(10), p.OnOrderStock, role)
		require.NotEmpty(t, p.Units, role)
	}
}

func TestGetProduct_RedactsCostForTillRoles(t *testing.T) {
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
	require.Equal(t, int64(40*1200), resp.Msg.Product.StockValuation)
	require.Equal(t, int64(1200), resp.Msg.Product.ReferenceCost)

	for _, role := range []string{"CASHIER", "APOTEKER"} {
		tillCtx := servicetest.CtxAs(context.Background(), role, ownerID)
		resp, err := svc.GetProduct(tillCtx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: pid}))
		require.NoError(t, err, role)
		p := resp.Msg.Product

		require.Zero(t, p.StockValuation, role)
		require.Zero(t, p.ReferenceCost, role)
		require.Empty(t, p.LastRestockDate, role)
		require.Empty(t, p.LastRestockSupplier, role)

		// Stock counts survive: the point of the page for a cashier is "do we
		// have it, and what does it sell for".
		require.Equal(t, int64(40), p.ReadyStock, role)
		require.Equal(t, int64(40), p.TotalStock, role)
		require.Equal(t, int64(2000), p.UnitPrice, role)
	}
}

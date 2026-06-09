package purchasing_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	productsvc "github.com/justmart/backend/internal/service/product"
	purchasing "github.com/justmart/backend/internal/service/purchasing"
	"github.com/justmart/backend/internal/service/servicetest"
	suppliersvc "github.com/justmart/backend/internal/service/supplier"
)

// poEnv bundles a fresh DB + the three purchasing services + supporting product
// and supplier services + an authenticated OWNER context. Every purchasing test
// gets its own throwaway SQLite DB so they can run in parallel.
type poEnv struct {
	db       *gorm.DB
	ownerID  string
	ctx      context.Context
	pos      *purchasing.PurchaseOrders
	receipts *purchasing.PurchaseReceipts
	payments *purchasing.PurchasePayments
	products *productsvc.ProductService
	supplier *suppliersvc.SupplierService
}

func newPOEnv(t *testing.T) *poEnv {
	t.Helper()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	return &poEnv{
		db:       gormDB,
		ownerID:  ownerID,
		ctx:      ctx,
		pos:      purchasing.NewPurchaseOrderService(gormDB),
		receipts: purchasing.NewPurchaseReceiptService(gormDB),
		payments: purchasing.NewPurchasePaymentService(gormDB),
		products: productsvc.NewProductService(gormDB),
		supplier: suppliersvc.NewSupplierService(gormDB),
	}
}

// seedSupplier creates a supplier via the real CreateSupplier handler and returns
// its id. Code must be unique per DB; tests pass a per-test code.
func (e *poEnv) seedSupplier(t *testing.T, code, name string) string {
	t.Helper()
	resp, err := e.supplier.CreateSupplier(e.ctx, connect.NewRequest(&inventoryifacev1.CreateSupplierRequest{
		Code: code,
		Name: name,
	}))
	require.NoError(t, err)
	return resp.Msg.Supplier.Id
}

// seedProduct creates a product (base unit + initial price) via the real
// CreateProduct handler and returns its id. CreateProduct FKs caller.UserID into
// product_prices.changed_by, so the env ctx must be authed (it is).
func (e *poEnv) seedProduct(t *testing.T, sku, name string, unitPrice int64) string {
	t.Helper()
	resp, err := e.products.CreateProduct(e.ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku:       sku,
		Name:      name,
		Unit:      "tablet",
		UnitPrice: unitPrice,
	}))
	require.NoError(t, err)
	return resp.Msg.Product.Id
}

// createPO creates a single-line PO (DRAFT) and returns the full proto order.
func (e *poEnv) createPO(t *testing.T, supplierID, productID string, qty int32, unitCost int64) *purchasingifacev1.PurchaseOrder {
	t.Helper()
	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supplierID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: productID, OrderedQty: qty, UnitCostPrice: unitCost},
		},
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Order.Items)
	return resp.Msg.Order
}

// sendPO transitions a DRAFT PO to SENT.
func (e *poEnv) sendPO(t *testing.T, poID string) {
	t.Helper()
	_, err := e.pos.SendPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.SendPurchaseOrderRequest{Id: poID}))
	require.NoError(t, err)
}

// receiveFull receives the full ordered qty of the (single-line) PO, creating a
// batch + stock movement. Returns the created receipt id.
func (e *poEnv) receiveFull(t *testing.T, poID, poItemID string, qty int32, batchNo string) string {
	t.Helper()
	resp, err := e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: poID,
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: poItemID, Qty: qty, ExpiryDate: "2099-12-31", BatchNumber: batchNo},
		},
	}))
	require.NoError(t, err)
	return resp.Msg.Receipt.Id
}

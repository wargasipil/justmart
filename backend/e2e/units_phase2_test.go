package e2e

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

// TestPurchaseOrder_BuyInUnits proves Phase-2 buy-in-units: a PO ordered in a
// larger purchasable unit (box ×100) stores its quantities in BASE units, the
// receive flow converts qty→base when creating the batch, over-receiving the
// remaining base qty is rejected, and the PO status advances correctly.
func TestPurchaseOrder_BuyInUnits(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	wh := makeWarehouse(env, t, ctx, fmt.Sprintf("BUY%d", uniq%100000))

	sup, err := env.Suppliers.CreateSupplier(ctx, authReq(env, t,
		&inventoryifacev1.CreateSupplierRequest{
			Name: "BuyInUnits supplier", Code: fmt.Sprintf("BIU%d", uniq%1000000),
		}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.Suppliers.ArchiveSupplier(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveSupplierRequest{Id: sup.Msg.Supplier.Id}))
	})

	// tablet (base) + box ×100 (purchasable).
	created, err := env.Products.CreateProduct(ctx, authReq(env, t,
		&inventoryifacev1.CreateProductRequest{
			Sku: fmt.Sprintf("biu-%d", uniq), Name: fmt.Sprintf("BIU-Med-%d", uniq),
			Unit: "tablet", UnitPrice: 700,
			Units: []*inventoryifacev1.ProductUnitInput{
				{Name: "box", Factor: 100, SellPrice: 60000, Sellable: true, Purchasable: true},
			},
		}))
	require.NoError(t, err)
	medID := created.Msg.Product.Id
	t.Cleanup(func() {
		_, _ = env.Products.ArchiveProduct(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveProductRequest{Id: medID}))
	})
	box := unitByName(created.Msg.Product.Units, "box")
	require.NotNil(t, box)

	// Order 5 box at Rp600/base (= Rp60.000/box, Rp300.000 line total).
	// PO is created in `wh` so the receipt lands batches there (POs are now
	// warehouse-scoped; the receipt uses the PO's warehouse, not the caller's).
	po, err := env.POs.CreatePurchaseOrder(ctx, whReq(env, t,
		&purchasingifacev1.CreatePurchaseOrderRequest{
			SupplierId: sup.Msg.Supplier.Id,
			Items: []*purchasingifacev1.PurchaseOrderItemInput{
				{ProductId: medID, ProductUnitId: box.Id, OrderedQty: 5, UnitCostPrice: 600},
			},
		}, wh))
	require.NoError(t, err)
	poID := po.Msg.Order.Id
	require.Len(t, po.Msg.Order.Items, 1)
	item := po.Msg.Order.Items[0]
	poItemID := item.Id
	require.Equal(t, int32(500), item.OrderedQty, "ordered_qty stored in base units (5 × 100)")
	require.Equal(t, "box", item.UnitName)
	require.Equal(t, int64(100), item.UnitFactor)
	require.Equal(t, box.Id, item.ProductUnitId)
	require.Equal(t, int64(300000), item.Subtotal, "base qty 500 × Rp600 = Rp300.000")

	_, err = env.POs.SendPurchaseOrder(ctx, authReq(env, t,
		&purchasingifacev1.SendPurchaseOrderRequest{Id: poID}))
	require.NoError(t, err)

	// Over-receive: 6 box = 600 base > 500 remaining → FailedPrecondition.
	_, err = env.Receipts.CreateReceipt(ctx, whReq(env, t,
		&purchasingifacev1.CreateReceiptRequest{
			PurchaseOrderId: poID,
			Lines: []*purchasingifacev1.ReceiveLineInput{
				{PurchaseOrderItemId: poItemID, ProductUnitId: box.Id, Qty: 6,
					ExpiryDate: "2099-12-31", BatchNumber: "BIU-OVER"},
			},
		}, wh))
	require.Error(t, err, "over-receiving the base remaining must fail")
	var cerr *connect.Error
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeFailedPrecondition, cerr.Code())

	// Receive 2 box = 200 base.
	rcv1, err := env.Receipts.CreateReceipt(ctx, whReq(env, t,
		&purchasingifacev1.CreateReceiptRequest{
			PurchaseOrderId: poID,
			Lines: []*purchasingifacev1.ReceiveLineInput{
				{PurchaseOrderItemId: poItemID, ProductUnitId: box.Id, Qty: 2,
					ExpiryDate: "2099-12-31", BatchNumber: "BIU-B1"},
			},
		}, wh))
	require.NoError(t, err)
	require.Len(t, rcv1.Msg.Receipt.Items, 1)
	r1 := rcv1.Msg.Receipt.Items[0]
	require.Equal(t, int32(200), r1.Qty, "receipt qty stored in base units (2 × 100)")
	require.Equal(t, "box", r1.UnitName)
	require.Equal(t, int64(100), r1.UnitFactor)
	require.Equal(t, int64(200), stockOf(env, t, ctx, r1.BatchId, wh), "batch holds 200 base units")

	got, err := env.POs.GetPurchaseOrder(ctx, authReq(env, t,
		&purchasingifacev1.GetPurchaseOrderRequest{Id: poID}))
	require.NoError(t, err)
	require.Equal(t, int32(200), got.Msg.Order.Items[0].ReceivedQty, "received_qty in base units")
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_PARTIALLY_RECEIVED, got.Msg.Order.Status)

	// Receive the remaining 3 box = 300 base → fully received.
	rcv2, err := env.Receipts.CreateReceipt(ctx, whReq(env, t,
		&purchasingifacev1.CreateReceiptRequest{
			PurchaseOrderId: poID,
			Lines: []*purchasingifacev1.ReceiveLineInput{
				{PurchaseOrderItemId: poItemID, ProductUnitId: box.Id, Qty: 3,
					ExpiryDate: "2099-12-31", BatchNumber: "BIU-B2"},
			},
		}, wh))
	require.NoError(t, err)
	require.Equal(t, int64(300), stockOf(env, t, ctx, rcv2.Msg.Receipt.Items[0].BatchId, wh))

	got, err = env.POs.GetPurchaseOrder(ctx, authReq(env, t,
		&purchasingifacev1.GetPurchaseOrderRequest{Id: poID}))
	require.NoError(t, err)
	require.Equal(t, int32(500), got.Msg.Order.Items[0].ReceivedQty)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_RECEIVED, got.Msg.Order.Status)
}

// TestProductUnitPriceHistory proves a unit's sell-price changes are versioned
// in product_unit_prices (one open row per unit; the old row is closed).
func TestProductUnitPriceHistory(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	created, err := env.Products.CreateProduct(ctx, authReq(env, t,
		&inventoryifacev1.CreateProductRequest{
			Sku: fmt.Sprintf("uph-%d", uniq), Name: fmt.Sprintf("UPH-Med-%d", uniq),
			Unit: "tablet", UnitPrice: 700,
		}))
	require.NoError(t, err)
	medID := created.Msg.Product.Id
	t.Cleanup(func() {
		_, _ = env.Products.ArchiveProduct(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveProductRequest{Id: medID}))
	})

	// Initial: one open row for the base unit at 700.
	h0, err := env.Products.ListProductUnitPrices(ctx, authReq(env, t,
		&inventoryifacev1.ListProductUnitPricesRequest{ProductId: medID}))
	require.NoError(t, err)
	require.Len(t, h0.Msg.Prices, 1)
	require.Equal(t, int64(700), h0.Msg.Prices[0].UnitSellPrice)
	require.Equal(t, int64(0), h0.Msg.Prices[0].EffectiveTo, "open row has effective_to 0")

	// Change the base price → a new version is recorded.
	_, err = env.Products.UpdateProduct(ctx, authReq(env, t,
		&inventoryifacev1.UpdateProductRequest{
			Id: medID, Name: created.Msg.Product.Name, Unit: "tablet", UnitPrice: 800,
		}))
	require.NoError(t, err)

	h1, err := env.Products.ListProductUnitPrices(ctx, authReq(env, t,
		&inventoryifacev1.ListProductUnitPricesRequest{ProductId: medID}))
	require.NoError(t, err)
	require.Len(t, h1.Msg.Prices, 2, "old (closed) + new (open)")
	open := 0
	for _, p := range h1.Msg.Prices {
		if p.EffectiveTo == 0 {
			open++
			require.Equal(t, int64(800), p.UnitSellPrice, "open row carries the new price")
		}
	}
	require.Equal(t, 1, open, "exactly one open row per unit")
}

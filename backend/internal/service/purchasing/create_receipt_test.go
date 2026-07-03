package purchasing_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func TestCreateReceipt_HappyPath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR", "Receipt supplier")
	prodID := e.seedProduct(t, "cr-sku", "CR product", 1000)
	po := e.createPO(t, supID, prodID, 10, 800)
	e.sendPO(t, po.Id) // must be SENT to receive

	resp, err := e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: po.Id,
		InvoiceNo:       "FAK-CR-1",
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: po.Items[0].Id, Qty: 4, ExpiryDate: "2099-12-31", BatchNumber: "CR-B1"},
		},
	}))
	require.NoError(t, err)
	r := resp.Msg.Receipt
	require.NotNil(t, r)
	require.NotEmpty(t, r.Id)
	require.NotEmpty(t, r.ReceiptNo) // RCV-YYYY-NNNN
	require.Equal(t, po.Id, r.PurchaseOrderId)
	require.Equal(t, "FAK-CR-1", r.InvoiceNo)
	require.Len(t, r.Items, 1)
	require.Equal(t, int32(4), r.Items[0].Qty)

	// Partial receive moves the PO to PARTIALLY_RECEIVED and bumps received_qty.
	got, err := e.pos.GetPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.GetPurchaseOrderRequest{Id: po.Id}))
	require.NoError(t, err)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_PARTIALLY_RECEIVED, got.Msg.Order.Status)
	require.Equal(t, int32(4), got.Msg.Order.Items[0].ReceivedQty)
}

// TestCreateReceipt_InheritsPOInvoiceWhenBlank pins the new behavior: the Receive
// dialog no longer asks for a faktur, so a blank request invoice_no inherits the
// PO's faktur (captured at PO create). An explicit value still wins — see
// TestCreateReceipt_HappyPath, which passes "FAK-CR-1" and asserts it survives.
func TestCreateReceipt_InheritsPOInvoiceWhenBlank(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR-INV", "Inherit-faktur supplier")
	prodID := e.seedProduct(t, "cr-inv-sku", "CR inherit product", 1000)

	// Create the PO WITH a faktur (as the create form does), then send it.
	poResp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		InvoiceNo:  "FAK-PO-9",
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 6, UnitCostPrice: 800},
		},
	}))
	require.NoError(t, err)
	po := poResp.Msg.Order
	e.sendPO(t, po.Id)

	// Receive with a BLANK invoice_no -> receipt inherits the PO's faktur.
	resp, err := e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: po.Id,
		// InvoiceNo intentionally omitted (blank).
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: po.Items[0].Id, Qty: 6, ExpiryDate: "2099-12-31", BatchNumber: "CR-INV-B1"},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "FAK-PO-9", resp.Msg.Receipt.InvoiceNo, "blank receipt faktur must inherit the PO's")
}

// TestCreateReceipt_DiscountedCostFlowsToBatch proves a per-line PO discount
// lowers inventory cost: with no receipt cost override, the created batch's
// cost_price is the NET per-base-unit cost (gross − line discount) / qty.
func TestCreateReceipt_DiscountedCostFlowsToBatch(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR-DISC", "Disc COGS supplier")
	prodID := e.seedProduct(t, "cr-disc-sku", "Disc COGS product", 1000)

	// 10 × 1000 gross = 10000, 12.5% (1250 bps) → net 8750 → net unit = 875.
	poResp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 10, UnitCostPrice: 1000, DiscountType: "PERCENT", DiscountValue: 1250},
		},
	}))
	require.NoError(t, err)
	po := poResp.Msg.Order
	e.sendPO(t, po.Id)

	// Receive full, NO cost override → batch must use the NET per-unit cost.
	_ = e.receiveFull(t, po.Id, po.Items[0].Id, 10, "CR-DISC-B1")

	var batch model.Batch
	require.NoError(t, e.db.Where("batch_number = ?", "CR-DISC-B1").First(&batch).Error)
	require.Equal(t, int64(875), batch.CostPrice) // 8750 / 10 — discount reflected in COGS
}

// TestRestock_CreateAndAccept is the end-to-end spec check (inventory > Restock:
// "ensure can create and accept restock"): create a restock order (PO), send it,
// then ACCEPT the full quantity (receipt) — the PO becomes RECEIVED and the stock
// actually lands (a batch + a PURCHASE movement for the accepted qty).
// Engine-agnostic → runs on both SQLite and Postgres via make test-unit-all.
func TestRestock_CreateAndAccept(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-RSTK", "Restock supplier")
	prodID := e.seedProduct(t, "rstk-sku", "Restock product", 1000)

	// Create the restock order — DRAFT, numbered, with the line.
	po := e.createPO(t, supID, prodID, 10, 800)
	require.NotEmpty(t, po.PoNo)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_DRAFT, po.Status)
	require.Len(t, po.Items, 1)

	// Send, then ACCEPT the full ordered quantity.
	e.sendPO(t, po.Id)
	e.receiveFull(t, po.Id, po.Items[0].Id, 10, "RSTK-B1")

	// Fully accepted: PO is RECEIVED and received_qty matches ordered.
	got, err := e.pos.GetPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.GetPurchaseOrderRequest{Id: po.Id}))
	require.NoError(t, err)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_RECEIVED, got.Msg.Order.Status)
	require.Equal(t, int32(10), got.Msg.Order.Items[0].ReceivedQty)

	// Stock landed: one batch + a PURCHASE movement summing to the accepted qty.
	var batchCount int64
	require.NoError(t, e.db.Model(&model.Batch{}).Where("product_id = ?", prodID).Count(&batchCount).Error)
	require.Equal(t, int64(1), batchCount)
	var onHand int64
	require.NoError(t, e.db.Model(&model.StockMovement{}).
		Joins("JOIN batches b ON b.id = stock_movements.batch_id").
		Where("b.product_id = ?", prodID).
		Select("COALESCE(SUM(stock_movements.qty), 0)").Scan(&onHand).Error)
	require.Equal(t, int64(10), onHand)
}

// TestCreateReceipt_RecordsRestock proves a receipt upserts the last-value
// restock row and appends a log row, with the NET unit cost + the PO line's
// discount; a second receipt updates the single last row and appends a 2nd log.
func TestCreateReceipt_RecordsRestock(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-RS", "Restock supplier")
	prodID := e.seedProduct(t, "rs-sku", "Restock product", 1000)

	// 10 × 1000 gross, 10% (1000 bps) line discount → net 9000 → net unit 900.
	poResp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 10, UnitCostPrice: 1000, DiscountType: "PERCENT", DiscountValue: 1000},
		},
	}))
	require.NoError(t, err)
	po := poResp.Msg.Order
	e.sendPO(t, po.Id)

	e.receiveFull(t, po.Id, po.Items[0].Id, 4, "RS-B1") // receive 4 of 10

	var lasts []model.ProductLastRestock
	require.NoError(t, e.db.Where("product_id = ? AND supplier_id = ?", prodID, supID).Find(&lasts).Error)
	require.Len(t, lasts, 1)
	require.Equal(t, int64(900), lasts[0].LastPrice) // NET per base unit
	require.Equal(t, int64(4), lasts[0].LastQty)
	require.Equal(t, "PERCENT", lasts[0].LastDiscountType)
	require.Equal(t, int64(1000), lasts[0].LastDiscountValue)
	require.False(t, lasts[0].LastArrivedAt.IsZero())
	require.False(t, lasts[0].LastCreatedAt.IsZero())

	var logs []model.ProductRestockLog
	require.NoError(t, e.db.Where("product_id = ?", prodID).Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, int64(900), logs[0].Price)
	require.Equal(t, int64(4), logs[0].Qty)
	require.NotNil(t, logs[0].ReceiptID)

	// Second receipt: last-value stays ONE row (updated qty), log appends → 2.
	e.receiveFull(t, po.Id, po.Items[0].Id, 6, "RS-B2")
	require.NoError(t, e.db.Where("product_id = ? AND supplier_id = ?", prodID, supID).Find(&lasts).Error)
	require.Len(t, lasts, 1)
	require.Equal(t, int64(6), lasts[0].LastQty) // updated to the latest receipt
	require.NoError(t, e.db.Where("product_id = ?", prodID).Find(&logs).Error)
	require.Len(t, logs, 2)
}

// TestCreateReceipt_RecordsRestockPerItemFlag proves a PO line's per-item
// discount flag propagates into both restock-history tables so the display can
// show "10% /item". A per-line discount (default false) is the negative control.
func TestCreateReceipt_RecordsRestockPerItemFlag(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-RS-PI", "Restock per-item supplier")
	prodID := e.seedProduct(t, "rs-pi-sku", "Restock per-item product", 1000)

	poResp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 10, UnitCostPrice: 1000, DiscountType: "PERCENT", DiscountValue: 1000, DiscountPerItem: true},
		},
	}))
	require.NoError(t, err)
	po := poResp.Msg.Order
	e.sendPO(t, po.Id)
	e.receiveFull(t, po.Id, po.Items[0].Id, 10, "RS-PI-B1")

	var last model.ProductLastRestock
	require.NoError(t, e.db.Where("product_id = ? AND supplier_id = ?", prodID, supID).First(&last).Error)
	require.True(t, last.LastDiscountPerItem, "last restock must carry the per-item flag")

	var log model.ProductRestockLog
	require.NoError(t, e.db.Where("product_id = ?", prodID).First(&log).Error)
	require.True(t, log.DiscountPerItem, "restock log must carry the per-item flag")
}

func TestCreateReceipt_OverReceiveRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR2", "Receipt supplier 2")
	prodID := e.seedProduct(t, "cr2-sku", "CR2 product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800)
	e.sendPO(t, po.Id)

	// Receiving more than ordered is rejected.
	_, err := e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: po.Id,
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: po.Items[0].Id, Qty: 6, ExpiryDate: "2099-12-31", BatchNumber: "CR2-B1"},
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestCreateReceipt_DraftPORejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR3", "Receipt supplier 3")
	prodID := e.seedProduct(t, "cr3-sku", "CR3 product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800) // still DRAFT — not sent

	_, err := e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: po.Id,
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: po.Items[0].Id, Qty: 1, ExpiryDate: "2099-12-31", BatchNumber: "CR3-B1"},
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestCreateReceipt_Unauthenticated(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	_, err := e.receipts.CreateReceipt(context.Background(), connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: "x",
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: "y", Qty: 1, ExpiryDate: "2099-12-31"},
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

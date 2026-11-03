package purchasing_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
	batchsvc "github.com/justmart/backend/internal/service/batch"
	"github.com/justmart/backend/internal/service/common"
	purchasing "github.com/justmart/backend/internal/service/purchasing"
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

// TestCreateReceipt_PPNCapitalizedIntoBatchCost proves PPN reaches inventory
// cost. It is unrecoverable for a non-PKP shop, so it is part of what the goods
// cost — leaving it out understated COGS and overstated every margin figure.
func TestCreateReceipt_PPNCapitalizedIntoBatchCost(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR-PPN", "PPN supplier")
	prodID := e.seedProduct(t, "cr-ppn-sku", "PPN product", 2000)

	// 10 × 1000 = 10000 net, PPN 11% → 1000/base becomes 1110/base.
	poResp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		PpnEnabled: true,
		PpnRate:    11,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 10, UnitCostPrice: 1000},
		},
	}))
	require.NoError(t, err)
	po := poResp.Msg.Order
	require.True(t, po.PpnEnabled)
	e.sendPO(t, po.Id)

	e.receiveFull(t, po.Id, po.Items[0].Id, 10, "CR-PPN-B1")

	var batch model.Batch
	require.NoError(t, e.db.Where("batch_number = ?", "CR-PPN-B1").First(&batch).Error)
	require.Equal(t, int64(1110), batch.CostPrice, "PPN must be capitalized into cost_price")

	// The restock history rides the same figure, so "last cost" and the margin
	// reference it feeds stay on one basis.
	var last model.ProductLastRestock
	require.NoError(t, e.db.Where("product_id = ? AND supplier_id = ?", prodID, supID).First(&last).Error)
	require.Equal(t, int64(1110), last.LastPrice)
}

// TestCreateReceipt_PPNStacksOnLineDiscount pins the order of operations: the
// line discount lowers the net first, THEN PPN scales it. Reversing the two
// would over-charge PPN on money never paid.
func TestCreateReceipt_PPNStacksOnLineDiscount(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR-PPND", "PPN+disc supplier")
	prodID := e.seedProduct(t, "cr-ppnd-sku", "PPN+disc product", 2000)

	// 10 × 1000 gross = 10000, −10% → 9000 net → 900/base, +11% → 999.
	poResp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		PpnEnabled: true,
		PpnRate:    11,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 10, UnitCostPrice: 1000, DiscountType: "PERCENT", DiscountValue: 1000},
		},
	}))
	require.NoError(t, err)
	po := poResp.Msg.Order
	e.sendPO(t, po.Id)
	e.receiveFull(t, po.Id, po.Items[0].Id, 10, "CR-PPND-B1")

	var batch model.Batch
	require.NoError(t, e.db.Where("batch_number = ?", "CR-PPND-B1").First(&batch).Error)
	require.Equal(t, int64(999), batch.CostPrice)
}

// TestCreateReceipt_PPNAppliesToCostOverride pins that PPN stays a uniform
// PO-level property: an explicit receipt cost is read on the same basis as the
// PO's entered costs (PPN-exclusive) and is scaled too, so the tax never
// depends on which lines an operator happened to override.
func TestCreateReceipt_PPNAppliesToCostOverride(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR-PPNO", "PPN override supplier")
	prodID := e.seedProduct(t, "cr-ppno-sku", "PPN override product", 2000)

	poResp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		PpnEnabled: true,
		PpnRate:    11,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 10, UnitCostPrice: 1000},
		},
	}))
	require.NoError(t, err)
	po := poResp.Msg.Order
	e.sendPO(t, po.Id)

	// Override at 2000/base → 2220 with PPN, ignoring the PO's 1000.
	_, err = e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: po.Id,
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: po.Items[0].Id, Qty: 10, ExpiryDate: "2099-12-31",
				BatchNumber: "CR-PPNO-B1", UnitCostPrice: 2000},
		},
	}))
	require.NoError(t, err)

	var batch model.Batch
	require.NoError(t, e.db.Where("batch_number = ?", "CR-PPNO-B1").First(&batch).Error)
	require.Equal(t, int64(2220), batch.CostPrice)
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
	requireReceiptToken(t, err, connect.CodeFailedPrecondition, "purchasing.receive_exceeds_remaining")
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
	requireReceiptToken(t, err, connect.CodeFailedPrecondition, "purchasing.po_not_receivable")
}

// A supplier that ships part of an order may leave whole products out. The
// Receive dialog sends only the lines that arrived, so the handler must accept
// a receipt naming a subset of the PO's lines and leave the rest outstanding.
func TestCreateReceipt_DeliveryMayOmitALine(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR4", "Receipt supplier 4")
	missingID := e.seedProduct(t, "cr4-a", "Did not arrive", 1000)
	arrivedID := e.seedProduct(t, "cr4-b", "Arrived", 1000)
	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: missingID, OrderedQty: 5, UnitCostPrice: 800},
			{ProductId: arrivedID, OrderedQty: 3, UnitCostPrice: 800},
		},
	}))
	require.NoError(t, err)
	po := resp.Msg.Order
	e.sendPO(t, po.Id)

	var arrivedItem string
	for _, it := range po.Items {
		if it.ProductId == arrivedID {
			arrivedItem = it.Id
		}
	}
	require.NotEmpty(t, arrivedItem)

	_, err = e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: po.Id,
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: arrivedItem, Qty: 3, ExpiryDate: "2099-12-31", BatchNumber: "CR4-B1"},
		},
	}))
	require.NoError(t, err)

	got := e.getPO(t, po.Id)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_PARTIALLY_RECEIVED, got.Status)
	for _, it := range got.Items {
		if it.ProductId == arrivedID {
			require.Equal(t, int32(3), it.ReceivedQty)
		} else {
			require.Equal(t, int32(0), it.ReceivedQty, "the omitted line stays outstanding")
		}
	}
}

// A receipt must not land on an order that was voided while it was in flight.
// The void below runs inside a transaction this test holds open, so its row
// lock on the PO is still held when CreateReceipt starts. With the PO locked
// at the top of CreateReceipt the receipt waits, then sees VOIDED and refuses.
// Without that lock it read SENT, landed the stock, and recomputePOStatus
// flipped the voided order back to PARTIALLY_RECEIVED. (Postgres is where this
// bites; SQLite's single-writer pool serializes the two either way.)
func TestCreateReceipt_WaitsForAConcurrentVoid(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR5", "Receipt supplier 5")
	prodID := e.seedProduct(t, "cr5-sku", "CR5 product", 1000)
	po := e.createPO(t, supID, prodID, 10, 800)
	e.sendPO(t, po.Id)

	tx := e.db.Begin()
	require.NoError(t, tx.Error)
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()
	_, err := purchasing.NewPurchaseOrderService(tx).VoidPurchaseOrder(e.ctx,
		connect.NewRequest(&purchasingifacev1.VoidPurchaseOrderRequest{Id: po.Id}))
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, err := e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
			PurchaseOrderId: po.Id,
			Lines: []*purchasingifacev1.ReceiveLineInput{
				{PurchaseOrderItemId: po.Items[0].Id, Qty: 4, ExpiryDate: "2099-12-31", BatchNumber: "CR5-B1"},
			},
		}))
		done <- err
	}()

	// Let the receipt reach the lock before the void commits. A shorter wait
	// can only make this test weaker (the receipt starts after the commit and
	// sees VOIDED regardless), never make it fail spuriously.
	time.Sleep(300 * time.Millisecond)
	require.NoError(t, tx.Commit().Error)
	committed = true

	select {
	case err := <-done:
		requireReceiptToken(t, err, connect.CodeFailedPrecondition, "purchasing.po_not_receivable")
	case <-time.After(10 * time.Second):
		t.Fatal("CreateReceipt did not return after the void committed")
	}
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_VOIDED, e.getPO(t, po.Id).Status)
}

// Malformed input that the dialog cannot produce but the API can still answers
// with stable tokens, so a caller sees a translated message rather than prose.
func TestCreateReceipt_InputErrorsAreTokens(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR6", "Receipt supplier 6")
	prodID := e.seedProduct(t, "cr6-sku", "CR6 product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800)
	e.sendPO(t, po.Id)

	cases := []struct {
		name       string
		receivedAt string
		itemID     string
		expiry     string
		token      string
	}{
		{"received_at", "31/12/2026", po.Items[0].Id, "2099-12-31", "purchasing.received_at_invalid"},
		{"expiry", "", po.Items[0].Id, "someday", "purchasing.expiry_invalid"},
		{"po item", "", "00000000-0000-0000-0000-000000000000", "2099-12-31", "purchasing.po_item_not_found"},
	}
	for _, c := range cases {
		_, err := e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
			PurchaseOrderId: po.Id,
			ReceivedAt:      c.receivedAt,
			Lines: []*purchasingifacev1.ReceiveLineInput{
				{PurchaseOrderItemId: c.itemID, Qty: 1, ExpiryDate: c.expiry},
			},
		}))
		requireReceiptToken(t, err, connect.CodeInvalidArgument, c.token)
	}
	// None of the refused receipts moved the order.
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_SENT, e.getPO(t, po.Id).Status)
}

// receiveOne receives qty of a single-line PO's only line with the given expiry
// and source, returning the receipt (or the error).
func (e *poEnv) receiveOne(poID, itemID, expiry string, src inventoryifacev1.ExpirySource) (*purchasingifacev1.PurchaseReceipt, error) {
	resp, err := e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: poID,
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: itemID, Qty: 1, ExpiryDate: expiry, ExpirySource: src},
		},
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.Receipt, nil
}

// The dialog reports where each line's date came from. A pre-filled default
// survives onto the lot as DEFAULT -- that is what puts it on the shelf-check
// worklist -- and NONE stores the placeholder whatever date was sent.
func TestCreateReceipt_RecordsExpirySource(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR7", "Receipt supplier 7")
	prodID := e.seedProduct(t, "cr7-sku", "CR7 product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800)
	e.sendPO(t, po.Id)

	def, err := e.receiveOne(po.Id, po.Items[0].Id, "2028-10-31", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_DEFAULT)
	require.NoError(t, err)
	none, err := e.receiveOne(po.Id, po.Items[0].Id, "", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_NONE)
	require.NoError(t, err)
	// The delivery line reports its lot's source, so the restock order can say
	// "does not expire" instead of printing the 2099 placeholder as a date.
	require.Equal(t, inventoryifacev1.ExpirySource_EXPIRY_SOURCE_DEFAULT, def.Items[0].ExpirySource)
	require.Equal(t, inventoryifacev1.ExpirySource_EXPIRY_SOURCE_NONE, none.Items[0].ExpirySource)
	listed, err := e.receipts.ListReceipts(e.ctx, connect.NewRequest(&purchasingifacev1.ListReceiptsRequest{
		PurchaseOrderId: po.Id,
	}))
	require.NoError(t, err)
	for _, r := range listed.Msg.Receipts {
		require.NotEqual(t, inventoryifacev1.ExpirySource_EXPIRY_SOURCE_UNSPECIFIED, r.Items[0].ExpirySource,
			"the list carries the source too")
	}

	var lot model.Batch
	require.NoError(t, e.db.First(&lot, "id = ?", def.Items[0].BatchId).Error)
	require.Equal(t, "DEFAULT", lot.ExpirySource)
	require.Equal(t, "2028-10-31", lot.ExpiryDate.Format("2006-01-02"))

	// A fresh struct: GORM's First on one that already holds a primary key adds
	// that key to the WHERE, and would look for the first lot again.
	var noExpiryLot model.Batch
	require.NoError(t, e.db.First(&noExpiryLot, "id = ?", none.Items[0].BatchId).Error)
	require.Equal(t, "NONE", noExpiryLot.ExpirySource)
	require.Equal(t, 2099, noExpiryLot.ExpiryDate.Year())
}

// Retail only warns about a past expiry (in the dialog); pharmacy refuses it,
// and refuses a prescription medicine recorded as never expiring.
func TestCreateReceipt_PharmacyExpiryRules(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR8", "Receipt supplier 8")
	prodID := e.seedProduct(t, "cr8-sku", "CR8 medicine", 1000)
	require.NoError(t, e.db.Model(&model.Product{}).Where("id = ?", prodID).
		Update("prescription_required", true).Error)
	po := e.createPO(t, supID, prodID, 10, 800)
	e.sendPO(t, po.Id)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	item := po.Items[0].Id

	_, err := e.receiveOne(po.Id, item, yesterday, inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	require.NoError(t, err, "retail: an expired date is the dialog's warning, not a refusal")
	_, err = e.receiveOne(po.Id, item, "", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_NONE)
	require.NoError(t, err, "retail: no pharmacy rule")

	require.NoError(t, common.SetBussinessType(e.ctx, e.db, common.BussinessTypePharmacyShop))
	_, err = e.receiveOne(po.Id, item, yesterday, inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	requireReceiptToken(t, err, connect.CodeFailedPrecondition, "purchasing.expiry_past")
	_, err = e.receiveOne(po.Id, item, "", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_NONE)
	requireReceiptToken(t, err, connect.CodeFailedPrecondition, "purchasing.expiry_required")

	today := time.Now().Format("2006-01-02")
	_, err = e.receiveOne(po.Id, item, today, inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	require.NoError(t, err, "expiring today has not passed yet")
}

// Correcting a lot's expiry later also corrects the receipt line's copy, so the
// restock order and the batches page never disagree about the same lot.
func TestSetBatchExpiry_UpdatesReceiptCopy(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR9", "Receipt supplier 9")
	prodID := e.seedProduct(t, "cr9-sku", "CR9 product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800)
	e.sendPO(t, po.Id)
	rcpt, err := e.receiveOne(po.Id, po.Items[0].Id, "2025-03-31", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	require.NoError(t, err)

	_, err = batchsvc.NewBatchService(e.db).SetBatchExpiry(e.ctx, connect.NewRequest(&inventoryifacev1.SetBatchExpiryRequest{
		BatchId: rcpt.Items[0].BatchId, ExpiryDate: "2027-03-31", Reason: "typo",
	}))
	require.NoError(t, err)

	got, err := e.receipts.GetReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.GetReceiptRequest{Id: rcpt.Id}))
	require.NoError(t, err)
	require.Equal(t, "2027-03-31", got.Msg.Receipt.Items[0].ExpiryDate)
}

func requireReceiptToken(t *testing.T, err error, code connect.Code, token string) {
	t.Helper()
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, code, ce.Code())
	require.Equal(t, token, ce.Message())
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

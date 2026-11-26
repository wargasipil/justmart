package purchasing_test

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// replaceReceipt corrects a receipt to a single line of qty (in the PO line's
// unit) with the given batch number and expiry.
func (e *poEnv) replaceReceipt(receiptID, reason, poItemID string, qty int32, batchNo, expiry string) (*purchasingifacev1.ReplaceReceiptResponse, error) {
	resp, err := e.receipts.ReplaceReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.ReplaceReceiptRequest{
		Id:     receiptID,
		Reason: reason,
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: poItemID, Qty: qty, BatchNumber: batchNo, ExpiryDate: expiry},
		},
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

// requireReceiptIntact asserts a refused replace changed nothing: the receipt is
// live, its lot still holds the full qty, the order still counts it, and no
// second receipt was written.
func (e *poEnv) requireReceiptIntact(t *testing.T, poID, receiptID, batchID string, qty int32) {
	t.Helper()
	var r model.PurchaseReceipt
	require.NoError(t, e.db.First(&r, "id = ?", receiptID).Error)
	require.Nil(t, r.VoidedAt, "the old delivery must not be cancelled")
	require.Nil(t, r.ReplacedByID)
	require.Equal(t, int64(1), e.countRows(t, "batches", "id = ?", batchID))
	require.Equal(t, int64(1), e.countRows(t, "purchase_receipts", "purchase_order_id = ?", poID))
	require.Equal(t, qty, e.getPO(t, poID).Items[0].ReceivedQty)
}

func inYears(n int) string { return time.Now().AddDate(n, 0, 0).Format("2006-01-02") }

// The happy path: the wrong delivery (10 of B-1, far-future expiry) becomes the
// corrected one (8 of B-1 re-used, a real expiry) in one call. The old receipt
// stays, voided and pointing at the new one; the new one has its own RCV number;
// the old lot is gone and the new lot holds exactly the corrected qty; the order
// counts 8 and reopens to PARTIALLY_RECEIVED.
func TestReplaceReceipt_CancelsAndReReceives(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-REP-1", "Acme")
	prod := e.seedProduct(t, "REP-SKU-1", "Paracetamol", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	old := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	_, oldBatch := e.receiptItem(t, old)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_RECEIVED, e.getPO(t, po.Id).Status)

	expiry := inYears(2)
	receivedAt := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	resp, err := e.receipts.ReplaceReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.ReplaceReceiptRequest{
		Id:         old,
		Reason:     "  salah jumlah  ",
		ReceivedAt: receivedAt,
		InvoiceNo:  "FKT-002",
		Note:       "koreksi",
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: po.Items[0].Id, Qty: 8, BatchNumber: "B-1", ExpiryDate: expiry},
		},
	}))
	require.NoError(t, err)
	created, replaced := resp.Msg.Receipt, resp.Msg.Replaced

	// The old document survives, voided, with the reason and the link.
	require.Equal(t, old, replaced.Id)
	require.NotZero(t, replaced.VoidedAt)
	require.Equal(t, "salah jumlah", replaced.VoidReason)
	require.Equal(t, e.ownerID, replaced.VoidedBy)
	require.Equal(t, created.Id, replaced.ReplacedById)
	require.Equal(t, created.ReceiptNo, replaced.ReplacedByNo)

	// The corrected delivery is a NEW receipt with its own number.
	require.NotEqual(t, old, created.Id)
	require.NotEmpty(t, created.ReceiptNo)
	require.NotEqual(t, replaced.ReceiptNo, created.ReceiptNo)
	require.Zero(t, created.VoidedAt)
	require.Empty(t, created.ReplacedById)
	require.Equal(t, receivedAt, created.ReceivedAt)
	require.Equal(t, "FKT-002", created.InvoiceNo)
	require.Equal(t, "koreksi", created.Note)
	require.Len(t, created.Items, 1)
	line := created.Items[0]
	require.Equal(t, int32(8), line.Qty)
	require.Equal(t, "B-1", line.BatchNumber)
	require.Equal(t, expiry, line.ExpiryDate)
	require.Equal(t, inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED, line.ExpirySource)

	// Stock: the old lot is gone (not zeroed), the new one holds the 8.
	require.Zero(t, e.countRows(t, "batches", "id = ?", oldBatch))
	require.Zero(t, e.countRows(t, "stock_movements", "batch_id = ?", oldBatch))
	require.NotEqual(t, oldBatch, line.BatchId)
	require.Equal(t, int64(8), e.batchOnHand(t, line.BatchId))
	require.Equal(t, int64(1), e.countRows(t, "batches", "product_id = ?", prod))

	// The order follows the corrected qty.
	got := e.getPO(t, po.Id)
	require.Equal(t, int32(8), got.Items[0].ReceivedQty)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_PARTIALLY_RECEIVED, got.Status)

	// One restock record, the new receipt's.
	require.Equal(t, int64(1), e.countRows(t, "product_restock_logs", "product_id = ?", prod))
	require.Equal(t, int64(1), e.countRows(t, "product_restock_logs", "receipt_id = ?", created.Id))
}

func TestReplaceReceipt_ReasonRequired(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-REP-2", "Acme")
	prod := e.seedProduct(t, "REP-SKU-2", "Amoxicillin", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	_, batchID := e.receiptItem(t, rcpt)

	_, err := e.replaceReceipt(rcpt, "   ", po.Items[0].Id, 8, "B-1", inYears(2))
	requireCancelToken(t, err, connect.CodeInvalidArgument, "purchasing.reason_required")
	e.requireReceiptIntact(t, po.Id, rcpt, batchID, 10)
}

// A consumed lot refuses exactly as Batalkan does -- the edit IS a cancel -- and
// nothing moves: not the lot, not the order, no new receipt.
func TestReplaceReceipt_ConsumedLotRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-REP-3", "Acme")
	prod := e.seedProduct(t, "REP-SKU-3", "Ibuprofen", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	_, batchID := e.receiptItem(t, rcpt)

	require.NoError(t, e.db.Create(&model.StockMovement{
		BatchID: batchID, Qty: -3, Type: "SALE", UserID: e.ownerID,
		WarehouseID: e.getPO(t, po.Id).WarehouseId,
	}).Error)

	_, err := e.replaceReceipt(rcpt, "salah jumlah", po.Items[0].Id, 8, "B-1", inYears(2))
	requireCancelToken(t, err, connect.CodeFailedPrecondition, "purchasing.receipt_lot_consumed")
	e.requireReceiptIntact(t, po.Id, rcpt, batchID, 10)
	require.Equal(t, int64(7), e.batchOnHand(t, batchID))
}

func TestReplaceReceipt_LotInStocktakeRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-REP-4", "Acme")
	prod := e.seedProduct(t, "REP-SKU-4", "Cetirizine", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	_, batchID := e.receiptItem(t, rcpt)

	warehouseID := e.getPO(t, po.Id).WarehouseId
	session := model.StocktakeSession{
		Name: "opname", Status: "DRAFT", CreatedBy: e.ownerID,
		WarehouseID: &warehouseID,
	}
	require.NoError(t, e.db.Create(&session).Error)
	require.NoError(t, e.db.Create(&model.StocktakeLine{
		SessionID: session.ID, BatchID: batchID, ExpectedQty: 10,
	}).Error)

	_, err := e.replaceReceipt(rcpt, "salah jumlah", po.Items[0].Id, 8, "B-1", inYears(2))
	requireCancelToken(t, err, connect.CodeFailedPrecondition, "purchasing.receipt_lot_in_stocktake")
	e.requireReceiptIntact(t, po.Id, rcpt, batchID, 10)
}

func TestReplaceReceipt_ClosedPORejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-REP-5", "Acme")
	prod := e.seedProduct(t, "REP-SKU-5", "Loratadine", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	_, batchID := e.receiptItem(t, rcpt)

	_, err := e.payments.PayPurchase(e.ctx, connect.NewRequest(&purchasingifacev1.PayPurchaseRequest{
		PurchaseOrderId: po.Id,
		Amount:          e.getPO(t, po.Id).OrderedTotal,
	}))
	require.NoError(t, err)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_CLOSED, e.getPO(t, po.Id).Status)

	_, err = e.replaceReceipt(rcpt, "salah jumlah", po.Items[0].Id, 8, "B-1", inYears(2))
	requireCancelToken(t, err, connect.CodeFailedPrecondition, "purchasing.receipt_cancel_po_closed")
	e.requireReceiptIntact(t, po.Id, rcpt, batchID, 10)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_CLOSED, e.getPO(t, po.Id).Status)
}

func TestReplaceReceipt_AlreadyVoidedRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-REP-6", "Acme")
	prod := e.seedProduct(t, "REP-SKU-6", "Ranitidine", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")

	_, err := e.cancelReceipt(rcpt, "salah input")
	require.NoError(t, err)

	_, err = e.replaceReceipt(rcpt, "salah jumlah", po.Items[0].Id, 8, "B-1", inYears(2))
	requireCancelToken(t, err, connect.CodeFailedPrecondition, "purchasing.receipt_already_voided")
	// The cancel's give-back is not repeated and no receipt was added.
	require.Equal(t, int32(0), e.getPO(t, po.Id).Items[0].ReceivedQty)
	require.Equal(t, int64(1), e.countRows(t, "purchase_receipts", "purchase_order_id = ?", po.Id))

	// Replacing twice: the first replacement voids the receipt, so the second
	// call on the SAME (now old) receipt is refused the same way.
	rcpt2 := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-2")
	_, err = e.replaceReceipt(rcpt2, "salah jumlah", po.Items[0].Id, 9, "B-2", inYears(2))
	require.NoError(t, err)
	_, err = e.replaceReceipt(rcpt2, "lagi", po.Items[0].Id, 7, "B-2", inYears(2))
	requireCancelToken(t, err, connect.CodeFailedPrecondition, "purchasing.receipt_already_voided")
	require.Equal(t, int32(9), e.getPO(t, po.Id).Items[0].ReceivedQty)
}

// The corrected qty may use what this delivery gave back plus what was still
// outstanding -- no more. PO 10, delivered 4 + 6: correcting the 4 to 5 would
// exceed the 4 left once it is given back. The refusal comes from the receive
// half, AFTER the cancel ran, and still leaves the old delivery intact: the
// whole edit is one transaction.
func TestReplaceReceipt_QtyAboveOutstandingRollsBack(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-REP-7", "Acme")
	prod := e.seedProduct(t, "REP-SKU-7", "Omeprazole", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	first := e.receiveFull(t, po.Id, po.Items[0].Id, 4, "B-1")
	e.receiveFull(t, po.Id, po.Items[0].Id, 6, "B-2")
	_, firstBatch := e.receiptItem(t, first)
	var before model.PurchaseReceipt
	require.NoError(t, e.db.First(&before, "id = ?", first).Error)
	firstNo := *before.ReceiptNo

	_, err := e.replaceReceipt(first, "salah jumlah", po.Items[0].Id, 5, "B-1", inYears(2))
	requireReceiptToken(t, err, connect.CodeFailedPrecondition, "purchasing.receive_exceeds_remaining")

	var r model.PurchaseReceipt
	require.NoError(t, e.db.First(&r, "id = ?", first).Error)
	require.Nil(t, r.VoidedAt)
	require.Nil(t, r.ReplacedByID)
	require.Equal(t, firstNo, *r.ReceiptNo)
	require.Equal(t, int64(4), e.batchOnHand(t, firstBatch))
	require.Equal(t, int64(2), e.countRows(t, "purchase_receipts", "purchase_order_id = ?", po.Id))
	got := e.getPO(t, po.Id)
	require.Equal(t, int32(10), got.Items[0].ReceivedQty)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_RECEIVED, got.Status)

	// Exactly the outstanding + this delivery's qty is fine.
	resp, err := e.replaceReceipt(first, "salah batch", po.Items[0].Id, 4, "B-1X", inYears(2))
	require.NoError(t, err)
	require.Equal(t, "B-1X", resp.Receipt.Items[0].BatchNumber)
	require.Equal(t, int32(10), e.getPO(t, po.Id).Items[0].ReceivedQty)
}

// The new lines are checked like any receive: a bad expiry refuses the edit
// and the cancel half is rolled back with it.
func TestReplaceReceipt_InvalidLineRollsBack(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-REP-8", "Acme")
	prod := e.seedProduct(t, "REP-SKU-8", "Metformin", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	_, batchID := e.receiptItem(t, rcpt)

	_, err := e.replaceReceipt(rcpt, "salah expiry", po.Items[0].Id, 10, "B-1", "someday")
	requireReceiptToken(t, err, connect.CodeInvalidArgument, "purchasing.expiry_invalid")
	e.requireReceiptIntact(t, po.Id, rcpt, batchID, 10)

	resp, err := e.receipts.ReplaceReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.ReplaceReceiptRequest{
		Id: rcpt, Reason: "kosong",
	}))
	require.Nil(t, resp)
	requireReceiptToken(t, err, connect.CodeInvalidArgument, "purchasing.lines_required")
	e.requireReceiptIntact(t, po.Id, rcpt, batchID, 10)
}

// The list shows the old delivery as cancelled and replaced by the new RCV
// number (one batched lookup), and the replacement is itself live and
// cancellable -- so it can be corrected again.
func TestReplaceReceipt_ListShowsReplacedByNo(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-REP-9", "Acme")
	prod := e.seedProduct(t, "REP-SKU-9", "Losartan", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	old := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")

	resp, err := e.replaceReceipt(old, "salah batch", po.Items[0].Id, 10, "B-9", inYears(3))
	require.NoError(t, err)
	created := resp.Receipt

	list := e.listReceipts(t, po.Id)
	require.Len(t, list, 2)
	byID := map[string]*purchasingifacev1.PurchaseReceipt{}
	for _, r := range list {
		byID[r.Id] = r
	}
	require.NotZero(t, byID[old].VoidedAt)
	require.Equal(t, created.Id, byID[old].ReplacedById)
	require.Equal(t, created.ReceiptNo, byID[old].ReplacedByNo)
	require.Equal(t, "purchasing.receipt_already_voided", byID[old].CancelBlockedReason)

	require.Zero(t, byID[created.Id].VoidedAt)
	require.Empty(t, byID[created.Id].ReplacedByNo)
	require.True(t, byID[created.Id].Cancellable)
	require.Empty(t, byID[created.Id].CancelBlockedReason)

	// GetReceipt carries the link too.
	got, err := e.receipts.GetReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.GetReceiptRequest{Id: old}))
	require.NoError(t, err)
	require.Equal(t, created.ReceiptNo, got.Msg.Receipt.ReplacedByNo)

	// A receipt cancelled outright (Batalkan) names no replacement.
	again, err := e.replaceReceipt(created.Id, "salah lagi", po.Items[0].Id, 10, "B-10", inYears(3))
	require.NoError(t, err)
	_, err = e.cancelReceipt(again.Receipt.Id, "batal")
	require.NoError(t, err)
	for _, r := range e.listReceipts(t, po.Id) {
		if r.Id == again.Receipt.Id {
			require.NotZero(t, r.VoidedAt)
			require.Empty(t, r.ReplacedById)
			require.Empty(t, r.ReplacedByNo)
		}
	}
}

// The corrected qty is in the purchase unit, like Terima barang: 3 boxes of 30
// lands 90 base units on the new lot.
func TestReplaceReceipt_QtyInPurchaseUnit(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	po := e.createPackPO(t, "REP-PACK", false, packLine)
	old := e.receiveFull(t, po.Id, po.Items[0].Id, 5, "REP-PACK-B1")
	require.Equal(t, int32(150), e.getPO(t, po.Id).Items[0].ReceivedQty)

	resp, err := e.replaceReceipt(old, "salah jumlah", po.Items[0].Id, 3, "REP-PACK-B1", inYears(2))
	require.NoError(t, err)
	line := resp.Receipt.Items[0]
	require.Equal(t, int32(90), line.Qty)
	require.Equal(t, int64(30), line.UnitFactor)
	require.Equal(t, int64(90), e.batchOnHand(t, line.BatchId))
	require.Equal(t, int32(90), e.getPO(t, po.Id).Items[0].ReceivedQty)
	// The exact pack cost is stamped on the new lot as on any receive.
	require.Equal(t, int64(65_000), e.batchByNumber(t, "REP-PACK-B1").CostPackPrice)
}

func TestReplaceReceipt_Unauthenticated(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	_, err := e.receipts.ReplaceReceipt(t.Context(), connect.NewRequest(&purchasingifacev1.ReplaceReceiptRequest{
		Id: "x", Reason: "y",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

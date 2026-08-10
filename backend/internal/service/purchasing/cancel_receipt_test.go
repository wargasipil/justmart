package purchasing_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func requireCancelToken(t *testing.T, err error, code connect.Code, token string) {
	t.Helper()
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, code, ce.Code())
	require.Equal(t, token, ce.Message())
}

func (e *poEnv) cancelReceipt(receiptID, reason string) (*purchasingifacev1.PurchaseReceipt, error) {
	resp, err := e.receipts.CancelReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CancelReceiptRequest{
		Id:     receiptID,
		Reason: reason,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.Receipt, nil
}

// listReceipts returns the PO's receipts (which is where cancellable is enriched).
func (e *poEnv) listReceipts(t *testing.T, poID string) []*purchasingifacev1.PurchaseReceipt {
	t.Helper()
	resp, err := e.receipts.ListReceipts(e.ctx, connect.NewRequest(&purchasingifacev1.ListReceiptsRequest{
		PurchaseOrderId: poID,
	}))
	require.NoError(t, err)
	return resp.Msg.Receipts
}

func (e *poEnv) countRows(t *testing.T, table, where string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, e.db.Raw("SELECT COUNT(*) FROM "+table+" WHERE "+where, args...).Scan(&n).Error)
	return n
}

// The happy path: an untouched receipt cancels cleanly — the lot and its
// movement are GONE (not zeroed), received_qty is given back, and the PO reverts
// to SENT because this was its only receipt.
func TestCancelReceipt_UndoesLotAndReopensPO(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-CAN-1", "Acme")
	prod := e.seedProduct(t, "CAN-SKU-1", "Paracetamol", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	_, batchID := e.receiptItem(t, rcpt)

	require.Equal(t, int64(10), e.batchOnHand(t, batchID))
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_RECEIVED, e.getPO(t, po.Id).Status)

	out, err := e.cancelReceipt(rcpt, "salah input")
	require.NoError(t, err)
	require.NotZero(t, out.VoidedAt)
	require.Equal(t, "salah input", out.VoidReason)
	require.Equal(t, e.ownerID, out.VoidedBy)

	// The lot ceases to exist — no ghost batch at qty 0.
	require.Zero(t, e.countRows(t, "batches", "id = ?", batchID))
	require.Zero(t, e.countRows(t, "stock_movements", "batch_id = ?", batchID))

	// The receipt document survives, with its number, and drops the batch link.
	require.Equal(t, int64(1), e.countRows(t, "purchase_receipts", "id = ?", rcpt))
	require.NotEmpty(t, out.ReceiptNo)
	require.Zero(t, e.countRows(t, "purchase_receipt_items",
		"purchase_receipt_id = ? AND batch_id IS NOT NULL", rcpt))

	// The PO reads as still awaiting the goods.
	got := e.getPO(t, po.Id)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_SENT, got.Status)
	require.Equal(t, int32(0), got.Items[0].ReceivedQty)
}

// Cancelling one of two receipts reverts only that delivery: the PO falls back
// to PARTIALLY_RECEIVED rather than all the way to SENT.
func TestCancelReceipt_PartialLeavesOtherReceipt(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-CAN-2", "Acme")
	prod := e.seedProduct(t, "CAN-SKU-2", "Amoxicillin", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	first := e.receiveFull(t, po.Id, po.Items[0].Id, 4, "B-1")
	second := e.receiveFull(t, po.Id, po.Items[0].Id, 6, "B-2")
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_RECEIVED, e.getPO(t, po.Id).Status)

	_, firstBatch := e.receiptItem(t, first)
	_, secondBatch := e.receiptItem(t, second)

	_, err := e.cancelReceipt(second, "dobel input")
	require.NoError(t, err)

	got := e.getPO(t, po.Id)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_PARTIALLY_RECEIVED, got.Status)
	require.Equal(t, int32(4), got.Items[0].ReceivedQty)
	require.Equal(t, int64(4), e.batchOnHand(t, firstBatch)) // untouched
	require.Zero(t, e.countRows(t, "batches", "id = ?", secondBatch))
}

// The core precondition: once anything has consumed the lot, only a return can
// unwind it. A sale leaves stock history that a cancel would have to invent.
func TestCancelReceipt_ConsumedLotRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-CAN-3", "Acme")
	prod := e.seedProduct(t, "CAN-SKU-3", "Ibuprofen", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	_, batchID := e.receiptItem(t, rcpt)

	require.NoError(t, e.db.Create(&model.StockMovement{
		BatchID: batchID, Qty: -3, Type: "SALE", UserID: e.ownerID,
		WarehouseID: e.getPO(t, po.Id).WarehouseId,
	}).Error)

	_, err := e.cancelReceipt(rcpt, "salah input")
	requireCancelToken(t, err, connect.CodeFailedPrecondition, "purchasing.receipt_lot_consumed")

	// Nothing moved: the lot, its stock and the PO are all as they were.
	require.Equal(t, int64(7), e.batchOnHand(t, batchID))
	require.Equal(t, int32(10), e.getPO(t, po.Id).Items[0].ReceivedQty)
	require.Equal(t, int64(1), e.countRows(t, "batches", "id = ?", batchID))
}

// A partial return already touched the lot, so the remainder is a return's job
// too — the untouched rule catches this without a separate guard.
func TestCancelReceipt_AfterReturnRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-CAN-4", "Acme")
	prod := e.seedProduct(t, "CAN-SKU-4", "Omeprazole", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	itemID, _ := e.receiptItem(t, rcpt)

	_, err := e.doReturn(po.Id, itemID, 2, "rusak")
	require.NoError(t, err)

	_, err = e.cancelReceipt(rcpt, "salah input")
	requireCancelToken(t, err, connect.CodeFailedPrecondition, "purchasing.receipt_lot_consumed")
}

// An open stocktake holds a NOT NULL FK to the lot. Refuse with a reason the
// operator can act on instead of letting the delete fail as a constraint error.
func TestCancelReceipt_LotInStocktakeRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-CAN-5", "Acme")
	prod := e.seedProduct(t, "CAN-SKU-5", "Cetirizine", 500)
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

	_, err := e.cancelReceipt(rcpt, "salah input")
	requireCancelToken(t, err, connect.CodeFailedPrecondition, "purchasing.receipt_lot_in_stocktake")
	require.Equal(t, int64(1), e.countRows(t, "batches", "id = ?", batchID))
}

// A settled PO stays settled: recomputePOStatus never reopens CLOSED, so a
// cancel there would leave it settled with a lowered received_qty.
func TestCancelReceipt_ClosedPORejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-CAN-6", "Acme")
	prod := e.seedProduct(t, "CAN-SKU-6", "Loratadine", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")

	_, err := e.payments.PayPurchase(e.ctx, connect.NewRequest(&purchasingifacev1.PayPurchaseRequest{
		PurchaseOrderId: po.Id,
		Amount:          e.getPO(t, po.Id).OrderedTotal,
	}))
	require.NoError(t, err)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_CLOSED, e.getPO(t, po.Id).Status)

	_, err = e.cancelReceipt(rcpt, "salah input")
	requireCancelToken(t, err, connect.CodeFailedPrecondition, "purchasing.receipt_cancel_po_closed")
}

func TestCancelReceipt_DoubleCancelRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-CAN-7", "Acme")
	prod := e.seedProduct(t, "CAN-SKU-7", "Ranitidine", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")

	_, err := e.cancelReceipt(rcpt, "salah input")
	require.NoError(t, err)

	_, err = e.cancelReceipt(rcpt, "sekali lagi")
	requireCancelToken(t, err, connect.CodeFailedPrecondition, "purchasing.receipt_already_voided")

	// received_qty must not be decremented twice.
	require.Equal(t, int32(0), e.getPO(t, po.Id).Items[0].ReceivedQty)
}

func TestCancelReceipt_ReasonRequired(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-CAN-8", "Acme")
	prod := e.seedProduct(t, "CAN-SKU-8", "Metformin", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")

	_, err := e.cancelReceipt(rcpt, "   ")
	requireCancelToken(t, err, connect.CodeInvalidArgument, "purchasing.reason_required")
}

// The last-restock row is an UPSERT, so cancelling must REBUILD it from the
// newest surviving log row. Reverting it in place (or leaving it) would show the
// cancelled receipt's cost on the product forever.
func TestCancelReceipt_RebuildsLastRestockFromPreviousReceipt(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-CAN-9", "Acme")
	prod := e.seedProduct(t, "CAN-SKU-9", "Simvastatin", 500)

	// First restock at 400/unit, then a second at 900/unit.
	po1 := e.createPO(t, sup, prod, 5, 400)
	e.sendPO(t, po1.Id)
	e.receiveFull(t, po1.Id, po1.Items[0].Id, 5, "B-1")

	po2 := e.createPO(t, sup, prod, 5, 900)
	e.sendPO(t, po2.Id)
	rcpt2 := e.receiveFull(t, po2.Id, po2.Items[0].Id, 5, "B-2")

	lastPrice := func() int64 {
		var p int64
		require.NoError(t, e.db.Raw(
			"SELECT last_price FROM product_last_restocks WHERE product_id = ? AND supplier_id = ?",
			prod, sup).Scan(&p).Error)
		return p
	}
	require.Equal(t, int64(900), lastPrice())
	require.Equal(t, int64(2), e.countRows(t, "product_restock_logs", "product_id = ?", prod))

	_, err := e.cancelReceipt(rcpt2, "harga salah")
	require.NoError(t, err)

	// Falls back to the first receipt's cost — not stale at 900, not zeroed.
	require.Equal(t, int64(400), lastPrice())
	require.Equal(t, int64(1), e.countRows(t, "product_restock_logs", "product_id = ?", prod))
}

// When the cancelled receipt was the only restock on record, the last-value row
// goes away entirely rather than lingering at a price nothing supports.
func TestCancelReceipt_DropsLastRestockWhenNoneRemain(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-CAN-10", "Acme")
	prod := e.seedProduct(t, "CAN-SKU-10", "Atorvastatin", 500)
	po := e.createPO(t, sup, prod, 5, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 5, "B-1")
	require.Equal(t, int64(1), e.countRows(t, "product_last_restocks", "product_id = ?", prod))

	_, err := e.cancelReceipt(rcpt, "salah supplier")
	require.NoError(t, err)

	require.Zero(t, e.countRows(t, "product_last_restocks", "product_id = ?", prod))
	require.Zero(t, e.countRows(t, "product_restock_logs", "product_id = ?", prod))
}

// ListReceipts precomputes cancellable so the UI can disable the action with a
// reason instead of failing on click. It must agree with what the handler does.
func TestCancelReceipt_CancellableSurfacedByList(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-CAN-11", "Acme")
	prod := e.seedProduct(t, "CAN-SKU-11", "Losartan", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	_, batchID := e.receiptItem(t, rcpt)

	list := e.listReceipts(t, po.Id)
	require.Len(t, list, 1)
	require.True(t, list[0].Cancellable)
	require.Empty(t, list[0].CancelBlockedReason)

	// Sell one unit → the list must flip to blocked, with the same token the
	// handler would have returned.
	require.NoError(t, e.db.Create(&model.StockMovement{
		BatchID: batchID, Qty: -1, Type: "SALE", UserID: e.ownerID,
		WarehouseID: e.getPO(t, po.Id).WarehouseId,
	}).Error)

	list = e.listReceipts(t, po.Id)
	require.False(t, list[0].Cancellable)
	require.Equal(t, "purchasing.receipt_lot_consumed", list[0].CancelBlockedReason)

	_, err := e.cancelReceipt(rcpt, "salah input")
	requireCancelToken(t, err, connect.CodeFailedPrecondition, list[0].CancelBlockedReason)
}

// A cancelled receipt stays in the list (its RCV number is already burned from
// the counter — a gap in the sequence is what an audit questions), marked voided
// and no longer cancellable.
func TestCancelReceipt_VoidedReceiptStaysListedAndBlocked(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-CAN-12", "Acme")
	prod := e.seedProduct(t, "CAN-SKU-12", "Bisoprolol", 500)
	po := e.createPO(t, sup, prod, 10, 400)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")

	_, err := e.cancelReceipt(rcpt, "salah input")
	require.NoError(t, err)

	list := e.listReceipts(t, po.Id)
	require.Len(t, list, 1)
	require.Equal(t, rcpt, list[0].Id)
	require.NotZero(t, list[0].VoidedAt)
	require.False(t, list[0].Cancellable)
	require.Equal(t, "purchasing.receipt_already_voided", list[0].CancelBlockedReason)
}

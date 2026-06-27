package purchasing_test

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// returnLine is a convenience for a single-line return request.
func (e *poEnv) doReturn(poID, receiptItemID string, qty int32, reason string) (*purchasingifacev1.PurchaseReturn, error) {
	resp, err := e.returns.CreatePurchaseReturn(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseReturnRequest{
		PurchaseOrderId: poID,
		Reason:          reason,
		Lines:           []*purchasingifacev1.ReturnLineInput{{PurchaseReceiptItemId: receiptItemID, Qty: qty}},
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.PurchaseReturn, nil
}

func requireRetToken(t *testing.T, err error, code connect.Code, token string) {
	t.Helper()
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, code, ce.Code())
	require.Equal(t, token, ce.Message())
}

func TestCreatePurchaseReturn_PartialReducesStockAndOutstanding(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-RET-1", "Acme")
	prod := e.seedProduct(t, "RET-SKU-1", "Paracetamol", 500)
	po := e.createPO(t, sup, prod, 10, 500) // ordered_total = 5000
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	itemID, batchID := e.receiptItem(t, rcpt)
	require.Equal(t, int64(10), e.batchOnHand(t, batchID))

	ret, err := e.doReturn(po.Id, itemID, 4, "rusak")
	require.NoError(t, err)
	require.Equal(t, int64(4*500), ret.RefundAmount)
	require.True(t, strings.HasPrefix(ret.ReturnNo, "RTN-"))

	require.Equal(t, int64(6), e.batchOnHand(t, batchID)) // 10 - 4 returned

	got := e.getPO(t, po.Id)
	require.Equal(t, int32(6), got.Items[0].ReceivedQty)              // net received drops
	require.Equal(t, int64(4*500), got.ReturnedAmount)
	require.Equal(t, int64(5000-0-2000), got.Outstanding)            // ordered - paid - returned
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_PARTIALLY_RECEIVED, got.Status) // reopened from RECEIVED
}

func TestCreatePurchaseReturn_FullReturnReopensToSent(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-RET-2", "Acme")
	prod := e.seedProduct(t, "RET-SKU-2", "Amoxicillin", 500)
	po := e.createPO(t, sup, prod, 10, 500)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	itemID, batchID := e.receiptItem(t, rcpt)

	_, err := e.doReturn(po.Id, itemID, 10, "semua rusak")
	require.NoError(t, err)
	require.Equal(t, int64(0), e.batchOnHand(t, batchID))

	got := e.getPO(t, po.Id)
	require.Equal(t, int32(0), got.Items[0].ReceivedQty)
	require.Equal(t, int64(0), got.Outstanding) // 5000 - 0 - 5000
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_SENT, got.Status)
}

func TestCreatePurchaseReturn_ClosedStaysClosedShowsCredit(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-RET-3", "Acme")
	prod := e.seedProduct(t, "RET-SKU-3", "Vitamin C", 500)
	po := e.createPO(t, sup, prod, 10, 500)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	itemID, _ := e.receiptItem(t, rcpt)
	// Pay in full -> CLOSED.
	_, err := e.payments.PayPurchase(e.ctx, connect.NewRequest(&purchasingifacev1.PayPurchaseRequest{
		PurchaseOrderId: po.Id, Amount: 5000,
	}))
	require.NoError(t, err)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_CLOSED, e.getPO(t, po.Id).Status)

	_, err = e.doReturn(po.Id, itemID, 4, "rusak")
	require.NoError(t, err)

	got := e.getPO(t, po.Id)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_CLOSED, got.Status) // stays CLOSED (settled)
	require.Equal(t, int64(-2000), got.Outstanding)                          // supplier owes a credit
}

func TestCreatePurchaseReturn_ExceedsOnHandRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-RET-4", "Acme")
	prod := e.seedProduct(t, "RET-SKU-4", "Ibuprofen", 500)
	po := e.createPO(t, sup, prod, 10, 500)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	itemID, batchID := e.receiptItem(t, rcpt)

	// Sell 8 of the 10 (a negative SALE movement) so only 2 remain on hand.
	require.NoError(t, e.db.Create(&model.StockMovement{
		BatchID: batchID, Qty: -8, Type: "SALE", UserID: e.ownerID,
		WarehouseID: e.getPO(t, po.Id).WarehouseId,
	}).Error)
	require.Equal(t, int64(2), e.batchOnHand(t, batchID))

	// Returning 5 exceeds the 2 still in stock.
	_, err := e.doReturn(po.Id, itemID, 5, "rusak")
	requireRetToken(t, err, connect.CodeFailedPrecondition, "purchasing.return_exceeds_on_hand")

	// Returning the 2 still on hand is fine.
	_, err = e.doReturn(po.Id, itemID, 2, "rusak")
	require.NoError(t, err)
	require.Equal(t, int64(0), e.batchOnHand(t, batchID))
}

func TestCreatePurchaseReturn_StatusGuard(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-RET-5", "Acme")
	prod := e.seedProduct(t, "RET-SKU-5", "Aspirin", 500)
	po := e.createPO(t, sup, prod, 10, 500) // DRAFT, no receipts

	// A DRAFT PO has no receipt items; use a bogus id — the status guard fires first.
	_, err := e.doReturn(po.Id, "00000000-0000-0000-0000-0000000000ff", 1, "x")
	requireRetToken(t, err, connect.CodeFailedPrecondition, "purchasing.po_not_returnable")

	e.sendPO(t, po.Id) // SENT, still nothing received
	_, err = e.doReturn(po.Id, "00000000-0000-0000-0000-0000000000ff", 1, "x")
	requireRetToken(t, err, connect.CodeFailedPrecondition, "purchasing.po_not_returnable")
}

func TestCreatePurchaseReturn_ReasonRequired(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-RET-6", "Acme")
	prod := e.seedProduct(t, "RET-SKU-6", "Cetirizine", 500)
	po := e.createPO(t, sup, prod, 10, 500)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	itemID, _ := e.receiptItem(t, rcpt)

	_, err := e.doReturn(po.Id, itemID, 1, "   ")
	requireRetToken(t, err, connect.CodeInvalidArgument, "purchasing.reason_required")
}

func TestCreatePurchaseReturn_ListAndGet(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-RET-7", "Acme")
	prod := e.seedProduct(t, "RET-SKU-7", "Loratadine", 500)
	po := e.createPO(t, sup, prod, 10, 500)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")
	itemID, _ := e.receiptItem(t, rcpt)

	r1, err := e.doReturn(po.Id, itemID, 2, "rusak")
	require.NoError(t, err)
	r2, err := e.doReturn(po.Id, itemID, 3, "expired")
	require.NoError(t, err)

	list, err := e.returns.ListPurchaseReturns(e.ctx, connect.NewRequest(&purchasingifacev1.ListPurchaseReturnsRequest{PurchaseOrderId: po.Id}))
	require.NoError(t, err)
	require.Len(t, list.Msg.Returns, 2)

	got, err := e.returns.GetPurchaseReturn(e.ctx, connect.NewRequest(&purchasingifacev1.GetPurchaseReturnRequest{Id: r1.Id}))
	require.NoError(t, err)
	require.Len(t, got.Msg.PurchaseReturn.Items, 1)
	require.Equal(t, int32(2), got.Msg.PurchaseReturn.Items[0].Qty)
	require.NotEqual(t, r1.ReturnNo, r2.ReturnNo) // incrementing counter
}

func TestCreatePurchaseReturn_ReturnableQtySurfaced(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "S-RET-8", "Acme")
	prod := e.seedProduct(t, "RET-SKU-8", "Omeprazole", 500)
	po := e.createPO(t, sup, prod, 10, 500)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "B-1")

	list, err := e.receipts.ListReceipts(e.ctx, connect.NewRequest(&purchasingifacev1.ListReceiptsRequest{PurchaseOrderId: po.Id}))
	require.NoError(t, err)
	require.Equal(t, int64(10), list.Msg.Receipts[0].Items[0].ReturnableQty)

	itemID, _ := e.receiptItem(t, rcpt)
	_, err = e.doReturn(po.Id, itemID, 4, "rusak")
	require.NoError(t, err)

	list, err = e.receipts.ListReceipts(e.ctx, connect.NewRequest(&purchasingifacev1.ListReceiptsRequest{PurchaseOrderId: po.Id}))
	require.NoError(t, err)
	require.Equal(t, int64(6), list.Msg.Receipts[0].Items[0].ReturnableQty) // 10 - 4
}

func TestCreatePurchaseReturn_Unauthenticated(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	_, err := e.returns.CreatePurchaseReturn(context.Background(), connect.NewRequest(&purchasingifacev1.CreatePurchaseReturnRequest{
		PurchaseOrderId: "x", Reason: "y", Lines: []*purchasingifacev1.ReturnLineInput{{PurchaseReceiptItemId: "z", Qty: 1}},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

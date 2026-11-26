package purchasing_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func TestUpdatePurchaseOrder_HappyPath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-UPO", "Update PO supplier")
	prodID := e.seedProduct(t, "upo-sku", "UPO product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800) // subtotal 4000

	// Full-replace the items + flip note. New: 10 × 600 = 6000.
	resp, err := e.pos.UpdatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.UpdatePurchaseOrderRequest{
		Id:   po.Id,
		Note: "revised",
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 10, UnitCostPrice: 600},
		},
	}))
	require.NoError(t, err)
	o := resp.Msg.Order
	require.Equal(t, "revised", o.Note)
	require.Equal(t, int64(6000), o.Subtotal)
	require.Equal(t, int64(6000), o.OrderedTotal)
	require.Len(t, o.Items, 1)
	require.Equal(t, int32(10), o.Items[0].OrderedQty)
	require.Equal(t, int64(600), o.Items[0].UnitCostPrice)
}

func TestUpdatePurchaseOrder_PercentLineDiscount(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-UPO-DISC", "Update PO disc supplier")
	prodID := e.seedProduct(t, "upo-disc-sku", "UPO disc product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800) // subtotal 4000, no discount

	// Full-replace with a percent-decimal line: 10 × 1000 = 10000, 12.5% → net 8750.
	resp, err := e.pos.UpdatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.UpdatePurchaseOrderRequest{
		Id:   po.Id,
		Note: "revised with discount",
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 10, UnitCostPrice: 1000, DiscountType: "PERCENT", DiscountValue: 1250},
		},
	}))
	require.NoError(t, err)
	o := resp.Msg.Order
	require.Equal(t, "revised with discount", o.Note)
	require.Len(t, o.Items, 1)
	require.Equal(t, "PERCENT", o.Items[0].DiscountType)
	require.Equal(t, int64(1250), o.Items[0].DiscountValue)
	require.Equal(t, int64(8750), o.Items[0].Subtotal)
	require.Equal(t, int64(8750), o.Subtotal)
	require.Equal(t, int64(8750), o.OrderedTotal)
}

func TestUpdatePurchaseOrder_PercentPerItemDiscount(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-UPO-PPI", "Update PO per-item supplier")
	prodID := e.seedProduct(t, "upo-ppi-sku", "UPO per-item product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800) // subtotal 4000, no discount

	// Replace with a percent-per-item line: 3 @ 105, 5%/item → net 300 (per-item
	// rounding; a line-level 5% would net 299).
	resp, err := e.pos.UpdatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.UpdatePurchaseOrderRequest{
		Id: po.Id,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 3, UnitCostPrice: 105, DiscountType: "PERCENT", DiscountValue: 500, DiscountPerItem: true},
		},
	}))
	require.NoError(t, err)
	o := resp.Msg.Order
	require.Len(t, o.Items, 1)
	require.True(t, o.Items[0].DiscountPerItem)
	require.Equal(t, int64(300), o.Items[0].Subtotal)
	require.Equal(t, int64(300), o.OrderedTotal)
}

// requireEditRefused asserts UpdatePurchaseOrder refused with FailedPrecondition
// and the given stable token (the contract frontend serverErrors.ts maps).
func requireEditRefused(t *testing.T, err error, token string) {
	t.Helper()
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, connect.CodeFailedPrecondition, ce.Code())
	require.Equal(t, token, ce.Message())
}

// editLines sends a one-line full replace: qty x unitCost per base unit.
func (e *poEnv) editLines(poID, prodID string, qty int32, unitCost int64) (*purchasingifacev1.PurchaseOrder, error) {
	resp, err := e.pos.UpdatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.UpdatePurchaseOrderRequest{
		Id:   poID,
		Note: "revised",
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: qty, UnitCostPrice: unitCost},
		},
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.Order, nil
}

// A SENT order that has never had a delivery is editable: the lines are
// replaced and the totals recomputed exactly as for a DRAFT, and the order
// stays SENT with its original sent_at (an edit is not a re-send).
func TestUpdatePurchaseOrder_SentWithoutReceiptsEditable(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-UPO-SENT", "Update PO sent supplier")
	prodID := e.seedProduct(t, "upo-sent-sku", "UPO sent product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800) // 4000
	e.sendPO(t, po.Id)
	sent := e.getPO(t, po.Id)
	require.NotZero(t, sent.SentAt)

	o, err := e.editLines(po.Id, prodID, 10, 600) // 6000
	require.NoError(t, err)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_SENT, o.Status)
	require.Equal(t, sent.SentAt, o.SentAt, "an edit must not re-stamp sent_at")
	require.Equal(t, "revised", o.Note)
	require.Equal(t, int64(6000), o.Subtotal)
	require.Equal(t, int64(6000), o.OrderedTotal)
	require.Equal(t, int64(6000), o.Outstanding)
	require.Len(t, o.Items, 1)
	require.Equal(t, int32(10), o.Items[0].OrderedQty)
	require.True(t, o.Editable, "still SENT with no receipt, so still editable")

	// Persisted, not just echoed.
	got := e.getPO(t, po.Id)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_SENT, got.Status)
	require.Equal(t, int64(6000), got.OrderedTotal)
}

// A SENT order can carry a payment already (canPay allows SENT). Lowering the
// total below it is allowed and leaves outstanding negative: a supplier credit,
// the same supported state a return can produce.
func TestUpdatePurchaseOrder_SentEditBelowPaidLeavesCredit(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-UPO-PAID", "Update PO paid supplier")
	prodID := e.seedProduct(t, "upo-paid-sku", "UPO paid product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800) // 4000
	e.sendPO(t, po.Id)
	_, err := e.payments.PayPurchase(e.ctx, connect.NewRequest(&purchasingifacev1.PayPurchaseRequest{
		PurchaseOrderId: po.Id,
		Amount:          3000,
	}))
	require.NoError(t, err)

	o, err := e.editLines(po.Id, prodID, 2, 800) // 1600
	require.NoError(t, err)
	require.Equal(t, int64(1600), o.OrderedTotal)
	require.Equal(t, int64(3000), o.PaidAmount)
	require.Equal(t, int64(-1400), o.Outstanding)
}

// Received everything, then returned everything: recomputePOStatus puts the
// order back to SENT, but the receipt (and the return hanging off it) still
// exists, so an edit is refused with its own token.
func TestUpdatePurchaseOrder_SentWithReceiptRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-UPO-RCV", "Update PO receipt supplier")
	prodID := e.seedProduct(t, "upo-rcv-sku", "UPO receipt product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 5, "UPO-RCV-B1")
	itemID, _ := e.receiptItem(t, rcpt)
	_, err := e.doReturn(po.Id, itemID, 5, "rusak")
	require.NoError(t, err)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_SENT, e.getPO(t, po.Id).Status)

	_, err = e.editLines(po.Id, prodID, 10, 600)
	requireEditRefused(t, err, "purchasing.edit_has_receipts")
}

// A cancelled receipt still blocks: its items keep the FK to the order's
// lines, which an edit deletes and re-creates.
func TestUpdatePurchaseOrder_SentAfterCancelledReceiptRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-UPO-CXL", "Update PO cancelled supplier")
	prodID := e.seedProduct(t, "upo-cxl-sku", "UPO cancelled product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800)
	e.sendPO(t, po.Id)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 5, "UPO-CXL-B1")

	// While the delivery stands the order is RECEIVED: past SENT entirely.
	_, err := e.editLines(po.Id, prodID, 10, 600)
	requireEditRefused(t, err, "purchasing.not_editable")

	_, err = e.cancelReceipt(rcpt, "salah input")
	require.NoError(t, err)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_SENT, e.getPO(t, po.Id).Status)

	_, err = e.editLines(po.Id, prodID, 10, 600)
	requireEditRefused(t, err, "purchasing.edit_has_receipts")
	// Refused means untouched.
	got := e.getPO(t, po.Id)
	require.Equal(t, int64(4000), got.OrderedTotal)
	require.Equal(t, int32(5), got.Items[0].OrderedQty)
}

// Anything that is neither DRAFT nor SENT is refused with not_editable.
func TestUpdatePurchaseOrder_PastSentRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-UPO2", "Update PO supplier 2")
	prodID := e.seedProduct(t, "upo2-sku", "UPO2 product", 1000)

	partial := e.createPO(t, supID, prodID, 5, 800)
	e.sendPO(t, partial.Id)
	_, err := e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: partial.Id,
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: partial.Items[0].Id, Qty: 2, ExpiryDate: "2099-12-31", BatchNumber: "UPO2-P1"},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_PARTIALLY_RECEIVED, e.getPO(t, partial.Id).Status)

	received := e.createPO(t, supID, prodID, 5, 800)
	e.sendPO(t, received.Id)
	e.receiveFull(t, received.Id, received.Items[0].Id, 5, "UPO2-R1")
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_RECEIVED, e.getPO(t, received.Id).Status)

	voided := e.createPO(t, supID, prodID, 5, 800)
	_, err = e.pos.VoidPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.VoidPurchaseOrderRequest{Id: voided.Id}))
	require.NoError(t, err)

	for name, id := range map[string]string{
		"partially received": partial.Id,
		"received":           received.Id,
		"voided":             voided.Id,
	} {
		_, err := e.editLines(id, prodID, 10, 600)
		requireEditRefused(t, err, "purchasing.not_editable")
		require.Equal(t, int64(4000), e.getPO(t, id).OrderedTotal, name)
	}
}

func TestUpdatePurchaseOrder_NotFound(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	_, err := e.pos.UpdatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.UpdatePurchaseOrderRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

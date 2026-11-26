package purchasing_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func TestGetPurchaseOrder_HappyPath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-GPO", "Get PO supplier")
	prodID := e.seedProduct(t, "gpo-sku", "GPO product", 1000)
	po := e.createPO(t, supID, prodID, 3, 500)

	resp, err := e.pos.GetPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.GetPurchaseOrderRequest{Id: po.Id}))
	require.NoError(t, err)
	got := resp.Msg.Order
	require.NotNil(t, got)
	require.Equal(t, po.Id, got.Id)
	require.Equal(t, po.PoNo, got.PoNo)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_DRAFT, got.Status)
	require.Len(t, got.Items, 1)
	require.Equal(t, prodID, got.Items[0].ProductId)
	require.Equal(t, int32(3), got.Items[0].OrderedQty)
}

func TestGetPurchaseOrder_NotFound(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)

	_, err := e.pos.GetPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.GetPurchaseOrderRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// editable is true exactly when UpdatePurchaseOrder would accept an edit, and
// every RPC that returns an order fills it: Create, Send, Get, Void.
func TestGetPurchaseOrder_EditableFlag(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-GPO-ED", "Get PO editable supplier")
	prodID := e.seedProduct(t, "gpo-ed-sku", "GPO editable product", 1000)

	// DRAFT: editable, on the create response and on Get.
	po := e.createPO(t, supID, prodID, 5, 800)
	require.True(t, po.Editable, "CreatePurchaseOrder response")
	require.True(t, e.getPO(t, po.Id).Editable, "DRAFT")

	// SENT, never delivered: still editable.
	sendResp, err := e.pos.SendPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.SendPurchaseOrderRequest{Id: po.Id}))
	require.NoError(t, err)
	require.True(t, sendResp.Msg.Order.Editable, "SendPurchaseOrder response")
	require.True(t, e.getPO(t, po.Id).Editable, "SENT without receipts")

	// A delivery: no longer editable, and not again after it is cancelled.
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 5, "GPO-ED-B1")
	require.False(t, e.getPO(t, po.Id).Editable, "RECEIVED")
	_, err = e.cancelReceipt(rcpt, "salah input")
	require.NoError(t, err)
	afterCancel := e.getPO(t, po.Id)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_SENT, afterCancel.Status)
	require.False(t, afterCancel.Editable, "SENT with a cancelled receipt")

	// VOIDED: never editable.
	draft := e.createPO(t, supID, prodID, 1, 100)
	voidResp, err := e.pos.VoidPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.VoidPurchaseOrderRequest{Id: draft.Id}))
	require.NoError(t, err)
	require.False(t, voidResp.Msg.Order.Editable, "VoidPurchaseOrder response")
	require.False(t, e.getPO(t, draft.Id).Editable, "VOIDED")
}

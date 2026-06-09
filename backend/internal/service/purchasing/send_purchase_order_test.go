package purchasing_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func TestSendPurchaseOrder_HappyPath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-SPO", "Send PO supplier")
	prodID := e.seedProduct(t, "spo-sku", "SPO product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800)

	resp, err := e.pos.SendPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.SendPurchaseOrderRequest{Id: po.Id}))
	require.NoError(t, err)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_SENT, resp.Msg.Order.Status)
	require.Greater(t, resp.Msg.Order.SentAt, int64(0))
}

func TestSendPurchaseOrder_AlreadySentRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-SPO2", "Send PO supplier 2")
	prodID := e.seedProduct(t, "spo2-sku", "SPO2 product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800)
	e.sendPO(t, po.Id) // DRAFT -> SENT

	// Sending a non-DRAFT PO again is rejected.
	_, err := e.pos.SendPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.SendPurchaseOrderRequest{Id: po.Id}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestSendPurchaseOrder_NotFound(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	_, err := e.pos.SendPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.SendPurchaseOrderRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

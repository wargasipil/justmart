package purchasing_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func TestVoidPurchaseOrder_HappyPath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-VPO", "Void PO supplier")
	prodID := e.seedProduct(t, "vpo-sku", "VPO product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800) // DRAFT

	resp, err := e.pos.VoidPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.VoidPurchaseOrderRequest{Id: po.Id}))
	require.NoError(t, err)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_VOIDED, resp.Msg.Order.Status)
}

func TestVoidPurchaseOrder_ReceivedRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-VPO2", "Void PO supplier 2")
	prodID := e.seedProduct(t, "vpo2-sku", "VPO2 product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800)
	e.sendPO(t, po.Id)
	e.receiveFull(t, po.Id, po.Items[0].Id, 5, "VPO-B1") // -> RECEIVED

	// Only DRAFT or SENT POs can be voided.
	_, err := e.pos.VoidPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.VoidPurchaseOrderRequest{Id: po.Id}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestVoidPurchaseOrder_NotFound(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	_, err := e.pos.VoidPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.VoidPurchaseOrderRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

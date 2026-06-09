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

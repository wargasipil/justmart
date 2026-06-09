package purchasing_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func TestGetReceipt_HappyPath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-GR", "Get receipt supplier")
	prodID := e.seedProduct(t, "gr-sku", "GR product", 1000)
	po := e.createPO(t, supID, prodID, 10, 800)
	e.sendPO(t, po.Id)
	rcvID := e.receiveFull(t, po.Id, po.Items[0].Id, 5, "GR-B1")

	resp, err := e.receipts.GetReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.GetReceiptRequest{Id: rcvID}))
	require.NoError(t, err)
	r := resp.Msg.Receipt
	require.NotNil(t, r)
	require.Equal(t, rcvID, r.Id)
	require.Equal(t, po.Id, r.PurchaseOrderId)
	require.NotEmpty(t, r.ReceiptNo)
	require.Len(t, r.Items, 1)
	require.Equal(t, int32(5), r.Items[0].Qty)
}

func TestGetReceipt_NotFound(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	_, err := e.receipts.GetReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.GetReceiptRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

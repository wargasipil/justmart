package purchasing_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func TestListReceipts_HappyPath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-LR", "List receipts supplier")
	prodID := e.seedProduct(t, "lr-sku", "LR product", 1000)
	po := e.createPO(t, supID, prodID, 10, 800)
	e.sendPO(t, po.Id)
	rcvID := e.receiveFull(t, po.Id, po.Items[0].Id, 4, "LR-B1")

	resp, err := e.receipts.ListReceipts(e.ctx, connect.NewRequest(&purchasingifacev1.ListReceiptsRequest{
		PurchaseOrderId: po.Id,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Receipts, 1)
	require.Equal(t, rcvID, resp.Msg.Receipts[0].Id)
	require.Equal(t, po.Id, resp.Msg.Receipts[0].PurchaseOrderId)
	require.Len(t, resp.Msg.Receipts[0].Items, 1)
}

func TestListReceipts_EmptyForUnknownPO(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	// No receipts exist for this id — the handler returns an empty list, no error.
	resp, err := e.receipts.ListReceipts(e.ctx, connect.NewRequest(&purchasingifacev1.ListReceiptsRequest{
		PurchaseOrderId: "00000000-0000-0000-0000-000000000000",
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Receipts)
}

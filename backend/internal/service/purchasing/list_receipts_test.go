package purchasing_test

import (
	"fmt"
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

// A PO received in several partial deliveries accumulates receipts, so the list
// pages. Items stay hydrated per page and returnable_qty is still enriched.
func TestListReceipts_Paginates(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-LRP", "List receipts paging supplier")
	prodID := e.seedProduct(t, "lrp-sku", "LRP product", 1000)
	po := e.createPO(t, supID, prodID, 10, 800)
	e.sendPO(t, po.Id)
	// Five partial receipts of 2 each.
	for i := 0; i < 5; i++ {
		e.receiveFull(t, po.Id, po.Items[0].Id, 2, fmt.Sprintf("LRP-B%d", i))
	}

	page := func(limit, offset int32) *purchasingifacev1.ListReceiptsResponse {
		t.Helper()
		resp, err := e.receipts.ListReceipts(e.ctx, connect.NewRequest(
			&purchasingifacev1.ListReceiptsRequest{
				PurchaseOrderId: po.Id, Limit: limit, Offset: offset,
			}))
		require.NoError(t, err)
		return resp.Msg
	}

	first := page(2, 0)
	require.Equal(t, int32(5), first.Total)
	require.Len(t, first.Receipts, 2)
	require.Len(t, first.Receipts[0].Items, 1) // Items preloaded on the page

	seen := map[string]bool{}
	for off := int32(0); off < first.Total; off += 2 {
		pg := page(2, off)
		require.Equal(t, first.Total, pg.Total)
		for _, r := range pg.Receipts {
			require.False(t, seen[r.Id], "receipt %s appeared on two pages", r.ReceiptNo)
			seen[r.Id] = true
		}
	}
	require.Len(t, seen, 5)
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

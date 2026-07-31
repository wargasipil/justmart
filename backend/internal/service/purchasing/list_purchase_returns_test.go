package purchasing_test

import (
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

// Returns against one PO accumulate, so the list pages. `total` is the full
// count for the filter, and the pages together cover every row exactly once.
func TestListPurchaseReturns_Paginates(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-LPR", "List returns supplier")
	prodID := e.seedProduct(t, "lpr-sku", "LPR product", 1000)
	po := e.createPO(t, supID, prodID, 20, 800)
	e.sendPO(t, po.Id)
	rcvID := e.receiveFull(t, po.Id, po.Items[0].Id, 20, "LPR-B1")
	itemID, _ := e.receiptItem(t, rcvID)

	// Five separate 1-unit returns off the same receipt line.
	for i := 0; i < 5; i++ {
		_, err := e.doReturn(po.Id, itemID, 1, fmt.Sprintf("damaged %d", i))
		require.NoError(t, err)
	}

	page := func(limit, offset int32) *purchasingifacev1.ListPurchaseReturnsResponse {
		t.Helper()
		resp, err := e.returns.ListPurchaseReturns(e.ctx, connect.NewRequest(
			&purchasingifacev1.ListPurchaseReturnsRequest{
				PurchaseOrderId: po.Id, Limit: limit, Offset: offset,
			}))
		require.NoError(t, err)
		return resp.Msg
	}

	first := page(2, 0)
	require.Equal(t, int32(5), first.Total)
	require.Len(t, first.Returns, 2)
	require.Len(t, first.Returns[0].Items, 1) // Items preloaded on the page

	seen := map[string]bool{}
	for off := int32(0); off < first.Total; off += 2 {
		pg := page(2, off)
		require.Equal(t, first.Total, pg.Total) // total ignores the window
		for _, r := range pg.Returns {
			require.False(t, seen[r.Id], "return %s appeared on two pages", r.Id)
			seen[r.Id] = true
		}
	}
	require.Len(t, seen, 5)

	// Past the end: empty page, total unchanged.
	require.Empty(t, page(2, 99).Returns)
	require.Equal(t, int32(5), page(2, 99).Total)
}

// An unpaged caller still gets everything (NormPage's default page size), so
// nothing that predates pagination regressed.
func TestListPurchaseReturns_DefaultLimitReturnsAll(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-LPRD", "List returns default supplier")
	prodID := e.seedProduct(t, "lprd-sku", "LPRD product", 1000)
	po := e.createPO(t, supID, prodID, 10, 800)
	e.sendPO(t, po.Id)
	rcvID := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "LPRD-B1")
	itemID, _ := e.receiptItem(t, rcvID)

	_, err := e.doReturn(po.Id, itemID, 2, "broken seal")
	require.NoError(t, err)

	resp, err := e.returns.ListPurchaseReturns(e.ctx, connect.NewRequest(
		&purchasingifacev1.ListPurchaseReturnsRequest{PurchaseOrderId: po.Id}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Returns, 1)
	require.Equal(t, int32(1), resp.Msg.Total)
}

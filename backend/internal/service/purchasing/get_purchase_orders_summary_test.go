package purchasing_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

// summaryFor is the common call shape — the stat row always asks with the same
// filters the list below it uses.
func summaryFor(
	t *testing.T, e *poEnv, req *purchasingifacev1.GetPurchaseOrdersSummaryRequest,
) *purchasingifacev1.GetPurchaseOrdersSummaryResponse {
	t.Helper()
	resp, err := e.pos.GetPurchaseOrdersSummary(e.ctx, connect.NewRequest(req))
	require.NoError(t, err)
	return resp.Msg
}

func TestGetPurchaseOrdersSummary_AggregatesAcrossOrders(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-SUM", "Summary supplier")
	prodA := e.seedProduct(t, "sum-a", "Summary A", 1000)
	prodB := e.seedProduct(t, "sum-b", "Summary B", 1000)

	// PO1 covers both products, PO2 re-orders A — so product_count must DEDUPE
	// to 2, not count 3 lines.
	_, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodA, OrderedQty: 4, UnitCostPrice: 700}, // 2800
			{ProductId: prodB, OrderedQty: 2, UnitCostPrice: 500}, // 1000
		},
	}))
	require.NoError(t, err)
	e.createPO(t, supID, prodA, 3, 700) // 2100

	got := summaryFor(t, e, &purchasingifacev1.GetPurchaseOrdersSummaryRequest{})
	require.Equal(t, int64(2), got.OrderCount)
	require.Equal(t, int64(2), got.ProductCount, "same product on two orders counts once")
	require.Equal(t, int64(9), got.ItemCount, "4 + 2 + 3 ordered")
	require.Equal(t, int64(5900), got.Total)

	// The row above the table and the table itself must describe one set.
	list, err := e.pos.ListPurchaseOrders(e.ctx, connect.NewRequest(&purchasingifacev1.ListPurchaseOrdersRequest{
		Limit: 100,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(list.Msg.Total), got.OrderCount)
}

func TestGetPurchaseOrdersSummary_ItemCountIsBaseUnits(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-SUMU", "Summary unit supplier")
	prodID, boxUnitID := e.seedProductWithBox(t, "sum-box", "Summary boxed", 100, 10)

	// Ordered in BOXES (3 × factor 10). item_count reports BASE units, matching
	// how stock is counted everywhere else in the app.
	_, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 3, UnitCostPrice: 80, ProductUnitId: boxUnitID},
		},
	}))
	require.NoError(t, err)

	got := summaryFor(t, e, &purchasingifacev1.GetPurchaseOrdersSummaryRequest{})
	require.Equal(t, int64(1), got.OrderCount)
	require.Equal(t, int64(30), got.ItemCount, "3 boxes x factor 10 = 30 base units")
	require.Equal(t, int64(2400), got.Total, "30 base units x 80 per base")
}

func TestGetPurchaseOrdersSummary_ScopedByStatusTab(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-SUMS", "Summary status supplier")
	prodID := e.seedProduct(t, "sum-st", "Summary status", 1000)
	e.createPO(t, supID, prodID, 5, 100) // stays DRAFT
	sent := e.createPO(t, supID, prodID, 2, 100)
	e.sendPO(t, sent.Id)

	sentSum := summaryFor(t, e, &purchasingifacev1.GetPurchaseOrdersSummaryRequest{
		Status: purchasingifacev1.POStatus_PO_STATUS_SENT,
	})
	require.Equal(t, int64(1), sentSum.OrderCount)
	require.Equal(t, int64(2), sentSum.ItemCount)
	require.Equal(t, int64(200), sentSum.Total)

	draftSum := summaryFor(t, e, &purchasingifacev1.GetPurchaseOrdersSummaryRequest{
		Status: purchasingifacev1.POStatus_PO_STATUS_DRAFT,
	})
	require.Equal(t, int64(1), draftSum.OrderCount)
	require.Equal(t, int64(5), draftSum.ItemCount)
	require.Equal(t, int64(500), draftSum.Total)

	// Unspecified status = the "All" tab: both orders.
	all := summaryFor(t, e, &purchasingifacev1.GetPurchaseOrdersSummaryRequest{})
	require.Equal(t, int64(2), all.OrderCount)
	require.Equal(t, int64(7), all.ItemCount)
}

func TestGetPurchaseOrdersSummary_ScopedBySupplierFilter(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supA := e.seedSupplier(t, "SUP-SUMA", "Summary A")
	supB := e.seedSupplier(t, "SUP-SUMB", "Summary B")
	prodID := e.seedProduct(t, "sum-sup", "Summary supplier product", 1000)
	e.createPO(t, supA, prodID, 6, 100)
	e.createPO(t, supB, prodID, 9, 100)

	got := summaryFor(t, e, &purchasingifacev1.GetPurchaseOrdersSummaryRequest{SupplierId: supA})
	require.Equal(t, int64(1), got.OrderCount)
	require.Equal(t, int64(1), got.ProductCount)
	require.Equal(t, int64(6), got.ItemCount)
	require.Equal(t, int64(600), got.Total)
}

func TestGetPurchaseOrdersSummary_NoMatchesIsZeroNotError(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supA := e.seedSupplier(t, "SUP-SUMZ", "Summary Z")
	supEmpty := e.seedSupplier(t, "SUP-SUMZE", "Summary Z empty")
	prodID := e.seedProduct(t, "sum-z", "Summary Z product", 1000)
	e.createPO(t, supA, prodID, 1, 100)

	// An empty match set must aggregate to zeros — the line-level query runs
	// against an empty sub-select, which is where a naive IN () would blow up.
	got := summaryFor(t, e, &purchasingifacev1.GetPurchaseOrdersSummaryRequest{SupplierId: supEmpty})
	require.Equal(t, int64(0), got.OrderCount)
	require.Equal(t, int64(0), got.ProductCount)
	require.Equal(t, int64(0), got.ItemCount)
	require.Equal(t, int64(0), got.Total)
}

func TestGetPurchaseOrdersSummary_Unauthenticated(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	_, err := e.pos.GetPurchaseOrdersSummary(
		context.Background(),
		connect.NewRequest(&purchasingifacev1.GetPurchaseOrdersSummaryRequest{}),
	)
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

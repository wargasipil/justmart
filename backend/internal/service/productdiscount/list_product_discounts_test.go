package productdiscount_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestListProductDiscounts(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	p1 := seedProduct(t, db, "lpd-1")
	p2 := seedProduct(t, db, "lpd-2")
	create(t, svc, p1, "PERCENT", false, 1000, 0)
	create(t, svc, p1, "FIXED", true, 500, 3)
	create(t, svc, p2, "PERCENT", false, 2000, 0)

	r, err := svc.ListProductDiscounts(ctx(), connect.NewRequest(&inventoryifacev1.ListProductDiscountsRequest{ProductId: p1}))
	require.NoError(t, err)
	require.Len(t, r.Msg.Discounts, 2)
	for _, d := range r.Msg.Discounts {
		require.Equal(t, p1, d.ProductId)
	}

	r2, err := svc.ListProductDiscounts(ctx(), connect.NewRequest(&inventoryifacev1.ListProductDiscountsRequest{ProductId: p2}))
	require.NoError(t, err)
	require.Len(t, r2.Msg.Discounts, 1)
}

// `total` is the product's full discount count, not the page length, and the
// pages together cover every row exactly once.
func TestListProductDiscounts_Paginates(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	pid := seedProduct(t, db, "lpd-page")
	for i := 0; i < 5; i++ {
		create(t, svc, pid, "FIXED", false, int64(100*(i+1)), 0)
	}

	page := func(limit, offset int32) *inventoryifacev1.ListProductDiscountsResponse {
		t.Helper()
		resp, err := svc.ListProductDiscounts(ctx(), connect.NewRequest(
			&inventoryifacev1.ListProductDiscountsRequest{
				ProductId: pid, Limit: limit, Offset: offset,
			}))
		require.NoError(t, err)
		return resp.Msg
	}

	first, second, third := page(2, 0), page(2, 2), page(2, 4)
	require.Equal(t, int32(5), first.Total)
	require.Len(t, first.Discounts, 2)
	require.Len(t, second.Discounts, 2)
	require.Len(t, third.Discounts, 1)

	// No row repeats or goes missing across the three pages.
	seen := map[string]bool{}
	for _, pg := range []*inventoryifacev1.ListProductDiscountsResponse{first, second, third} {
		for _, d := range pg.Discounts {
			require.False(t, seen[d.Id], "discount %s appeared on two pages", d.Id)
			seen[d.Id] = true
		}
	}
	require.Len(t, seen, 5)
}

// An omitted limit falls back to NormPage's default page size, so callers that
// predate pagination keep seeing a full (small) ladder.
func TestListProductDiscounts_DefaultLimitReturnsAll(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	pid := seedProduct(t, db, "lpd-def")
	create(t, svc, pid, "PERCENT", false, 1000, 0)
	create(t, svc, pid, "FIXED", true, 500, 3)

	r, err := svc.ListProductDiscounts(ctx(), connect.NewRequest(
		&inventoryifacev1.ListProductDiscountsRequest{ProductId: pid}))
	require.NoError(t, err)
	require.Len(t, r.Msg.Discounts, 2)
	require.Equal(t, int32(2), r.Msg.Total)
}

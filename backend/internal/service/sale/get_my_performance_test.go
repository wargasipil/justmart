package sale_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
)

// TestGetMyPerformance_SelfScopeAndTotals: the caller's own COMPLETED sales are
// summed; another cashier's sales are never counted.
func TestGetMyPerformance_SelfScopeAndTotals(t *testing.T) {
	t.Parallel()
	svc, ownerCtx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "mp-sku", "Paracetamol", 2000)
	seedStock(t, db, productID, ownerID, 100)

	// Owner sale (must NOT appear in the cashier's performance).
	ownerSale := startDraft(t, svc, ownerCtx)
	completeOne(t, svc, ownerCtx, productID, ownerSale, 4, 10000)

	// Cashier: two sales — 3 units (6000) + 2 units (4000) = 10000 / 5 units / 2 sales.
	cashierID := seedCashier(t, db, "mp-cashier@x.test")
	cashierCtx := servicetest.CtxAs(context.Background(), "CASHIER", cashierID)
	s1 := startDraft(t, svc, cashierCtx)
	completeOne(t, svc, cashierCtx, productID, s1, 3, 10000)
	s2 := startDraft(t, svc, cashierCtx)
	completeOne(t, svc, cashierCtx, productID, s2, 2, 10000)

	resp, err := svc.GetMyPerformance(cashierCtx, connect.NewRequest(&posifacev1.GetMyPerformanceRequest{}))
	require.NoError(t, err)
	require.Equal(t, int64(10000), resp.Msg.TotalRevenue)
	require.Equal(t, int64(2), resp.Msg.TotalSalesCount)
	require.Equal(t, int64(5), resp.Msg.TotalItemsSold)

	// Buckets sum to the totals (and carry no profit/cost — the type has no such field).
	var rev, cnt, items int64
	for _, b := range resp.Msg.Buckets {
		rev += b.Revenue
		cnt += b.SalesCount
		items += b.ItemsSold
	}
	require.Equal(t, resp.Msg.TotalRevenue, rev)
	require.Equal(t, resp.Msg.TotalSalesCount, cnt)
	require.Equal(t, resp.Msg.TotalItemsSold, items)
}

// TestGetMyPerformance_ReconcilesWithSnapshot: GetMyPerformance totals match the
// cashier's GetTodaySnapshot for the same sales (same revenue/qty definitions).
func TestGetMyPerformance_ReconcilesWithSnapshot(t *testing.T) {
	t.Parallel()
	svc, _, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "mp-recon-sku", "Paracetamol", 2000)
	seedStock(t, db, productID, ownerID, 100)
	cashierID := seedCashier(t, db, "mp-recon@x.test")
	cashierCtx := servicetest.CtxAs(context.Background(), "CASHIER", cashierID)
	sale := startDraft(t, svc, cashierCtx)
	completeOne(t, svc, cashierCtx, productID, sale, 3, 10000)

	perf, err := svc.GetMyPerformance(cashierCtx, connect.NewRequest(&posifacev1.GetMyPerformanceRequest{}))
	require.NoError(t, err)
	snap, err := svc.GetTodaySnapshot(cashierCtx, connect.NewRequest(&posifacev1.GetTodaySnapshotRequest{
		CashierUserId: cashierID,
	}))
	require.NoError(t, err)
	require.Equal(t, snap.Msg.Revenue, perf.Msg.TotalRevenue)
	require.Equal(t, snap.Msg.SaleCount, perf.Msg.TotalSalesCount)
	require.Equal(t, snap.Msg.ItemsSold, perf.Msg.TotalItemsSold)
}

// TestGetMyPerformance_EmptyBucketsZeroed: a past range with no sales yields
// zero-filled, chronological buckets.
func TestGetMyPerformance_EmptyBucketsZeroed(t *testing.T) {
	t.Parallel()
	svc, _, db, _ := newSaleSvc(t)
	cashierID := seedCashier(t, db, "mp-empty@x.test")
	cashierCtx := servicetest.CtxAs(context.Background(), "CASHIER", cashierID)

	from := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 3)
	resp, err := svc.GetMyPerformance(cashierCtx, connect.NewRequest(&posifacev1.GetMyPerformanceRequest{
		FromUnix:    from.Unix(),
		ToUnix:      to.Unix(),
		Granularity: posifacev1.PerformanceGranularity_PERFORMANCE_GRANULARITY_DAY,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Buckets)
	require.Equal(t, int64(0), resp.Msg.TotalRevenue)
	require.Equal(t, int64(0), resp.Msg.TotalSalesCount)
	require.Equal(t, int64(0), resp.Msg.TotalItemsSold)
	for i, b := range resp.Msg.Buckets {
		require.Equal(t, int64(0), b.Revenue)
		require.Equal(t, int64(0), b.SalesCount)
		require.Equal(t, int64(0), b.ItemsSold)
		if i > 0 {
			require.Less(t, resp.Msg.Buckets[i-1].DayKey, b.DayKey) // chronological
		}
	}
}

// TestGetMyPerformance_WeekGranularity: WEEK buckets still total the same sales.
func TestGetMyPerformance_WeekGranularity(t *testing.T) {
	t.Parallel()
	svc, _, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "mp-week-sku", "Paracetamol", 2000)
	seedStock(t, db, productID, ownerID, 100)
	cashierID := seedCashier(t, db, "mp-week@x.test")
	cashierCtx := servicetest.CtxAs(context.Background(), "CASHIER", cashierID)
	sale := startDraft(t, svc, cashierCtx)
	completeOne(t, svc, cashierCtx, productID, sale, 3, 10000)

	resp, err := svc.GetMyPerformance(cashierCtx, connect.NewRequest(&posifacev1.GetMyPerformanceRequest{
		Granularity: posifacev1.PerformanceGranularity_PERFORMANCE_GRANULARITY_WEEK,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(6000), resp.Msg.TotalRevenue)
	require.Equal(t, int64(1), resp.Msg.TotalSalesCount)
	require.Equal(t, int64(3), resp.Msg.TotalItemsSold)
}

func TestGetMyPerformance_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newSaleSvc(t)

	_, err := svc.GetMyPerformance(context.Background(), connect.NewRequest(&posifacev1.GetMyPerformanceRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

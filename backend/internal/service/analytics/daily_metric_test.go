package analytics_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	analyticsifacev1 "github.com/justmart/backend/gen/analytics_iface/v1"
	"github.com/justmart/backend/internal/model"
	analyticssvc "github.com/justmart/backend/internal/service/analytics"
	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/service/servicetest"
)

// TestDailyMetric_EmptyRange is the happy path: an OWNER over a 30-day range with
// no sales seeded gets a fully-formed (but zero-valued) response. The day buckets
// are enumerated from the range, and each requested metric block is present with
// a zero entry per bucket.
func TestDailyMetric_EmptyRange(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ctx := servicetest.OwnerCtx(context.Background(), servicetest.EnsureOwner(t, gormDB, cfg))
	svc := analyticssvc.NewAnalyticsService(gormDB)

	now := time.Now()
	resp, err := svc.DailyMetric(ctx, connect.NewRequest(&analyticsifacev1.DailyMetricRequest{
		MetricTypes: []analyticsifacev1.MetricType{
			analyticsifacev1.MetricType_METRIC_TYPE_ORDER,
			analyticsifacev1.MetricType_METRIC_TYPE_STOCK,
		},
		Granularity: analyticsifacev1.Granularity_GRANULARITY_DAY,
		Filter: &analyticsifacev1.Filter{
			FromUnix: now.AddDate(0, 0, -7).Unix(),
			ToUnix:   now.Unix(),
		},
	}))
	require.NoError(t, err)

	// 7-day range -> at least 7 day buckets enumerated.
	require.NotEmpty(t, resp.Msg.Days)
	require.GreaterOrEqual(t, len(resp.Msg.Days), 7)

	// Both metric blocks present (requested) and every day key has an entry.
	require.NotNil(t, resp.Msg.Order)
	require.NotNil(t, resp.Msg.Stock)
	for _, day := range resp.Msg.Days {
		o, ok := resp.Msg.Order.Data[day]
		require.True(t, ok, "order block missing day %s", day)
		// No sales -> all order metrics are zero.
		require.Equal(t, int64(0), o.Terjual)
		require.Equal(t, int64(0), o.Hpp)
		require.Equal(t, int64(0), o.Profit)

		s, ok := resp.Msg.Stock.Data[day]
		require.True(t, ok, "stock block missing day %s", day)
		// No stock movements -> ready is zero; no open POs -> ongoing zero.
		require.Equal(t, int64(0), s.Ready)
		require.Equal(t, int64(0), s.Ongoing)
	}
}

// TestDailyMetric_TerjualIncludesServiceFee is the regression guard for the
// biaya_jasa fix: daily revenue must sum the sale-level total (which includes
// the resep service fee), NOT just the sale_items subtotal. A completed sale
// with subtotal 10_000 + biaya_jasa 5_000 (total 15_000) must report terjual
// 15_000 for today. (The pre-fix query summed sale_items.line_total and would
// have reported 10_000 — or 0 for an item-less sale.)
func TestDailyMetric_TerjualIncludesServiceFee(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	svc := analyticssvc.NewAnalyticsService(gormDB)

	// Resolve the migration-seeded default (MAIN) warehouse the owner sells in.
	var wh model.Warehouse
	require.NoError(t, gormDB.Where("is_default").First(&wh).Error)

	now := time.Now()
	whID := wh.ID
	sale := model.Sale{
		CashierUserID: ownerID,
		WarehouseID:   &whID,
		Subtotal:      10_000,
		BiayaJasa:     5_000,
		Total:         15_000, // subtotal - cart_discount + biaya_jasa
		PaidAmount:    15_000,
		Status:        common.SaleStatusCompleted,
		CompletedAt:   &now,
	}
	require.NoError(t, gormDB.Create(&sale).Error)

	resp, err := svc.DailyMetric(ctx, connect.NewRequest(&analyticsifacev1.DailyMetricRequest{
		MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
		Granularity: analyticsifacev1.Granularity_GRANULARITY_DAY,
		Filter: &analyticsifacev1.Filter{
			FromUnix: now.AddDate(0, 0, -1).Unix(),
			ToUnix:   now.AddDate(0, 0, 1).Unix(),
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Order)

	// Sum terjual across the (small) range — the single sale lands in today's bucket.
	var totalTerjual int64
	for _, o := range resp.Msg.Order.Data {
		totalTerjual += o.Terjual
	}
	require.Equal(t, int64(15_000), totalTerjual, "daily terjual must include the biaya_jasa service fee")
}

// TestDailyMetric_EmptyMetricTypes asserts the validation precondition: an empty
// metric_types list is rejected with InvalidArgument (before any auth/DB work).
func TestDailyMetric_EmptyMetricTypes(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ctx := servicetest.OwnerCtx(context.Background(), servicetest.EnsureOwner(t, gormDB, cfg))
	svc := analyticssvc.NewAnalyticsService(gormDB)

	_, err := svc.DailyMetric(ctx, connect.NewRequest(&analyticsifacev1.DailyMetricRequest{
		MetricTypes: nil, // empty -> InvalidArgument
		Granularity: analyticsifacev1.Granularity_GRANULARITY_DAY,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestDailyMetric_Unauthenticated asserts that, with a valid request but no
// principal in context, the handler returns Unauthenticated (validation passes,
// then MustPrincipal fails).
func TestDailyMetric_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := analyticssvc.NewAnalyticsService(gormDB)

	_, err := svc.DailyMetric(context.Background(), connect.NewRequest(&analyticsifacev1.DailyMetricRequest{
		MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
		Granularity: analyticsifacev1.Granularity_GRANULARITY_DAY,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

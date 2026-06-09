package analytics_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	analyticsifacev1 "github.com/justmart/backend/gen/analytics_iface/v1"
	analyticssvc "github.com/justmart/backend/internal/service/analytics"
	"github.com/justmart/backend/internal/service/servicetest"
)

// TestUserMetric_NoSales is the happy path: with no completed sales, no user has
// any order aggregate, so the page is empty and total is 0.
func TestUserMetric_NoSales(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ctx := servicetest.OwnerCtx(context.Background(), servicetest.EnsureOwner(t, gormDB, cfg))
	svc := analyticssvc.NewAnalyticsService(gormDB)

	now := time.Now()
	resp, err := svc.UserMetric(ctx, connect.NewRequest(&analyticsifacev1.UserMetricRequest{
		MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
		Filter: &analyticsifacev1.Filter{
			FromUnix: now.AddDate(0, 0, -30).Unix(),
			ToUnix:   now.Unix(),
		},
		Limit:  25,
		Offset: 0,
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.UserIds)
	require.Equal(t, int32(0), resp.Msg.Total)
	// No ids -> handler returns before populating the order block.
	require.Nil(t, resp.Msg.Order)
}

// TestUserMetric_StockMetricRejected asserts the documented USER-dimension rule:
// requesting STOCK in metric_types returns InvalidArgument (stock has no meaning
// per user).
func TestUserMetric_StockMetricRejected(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ctx := servicetest.OwnerCtx(context.Background(), servicetest.EnsureOwner(t, gormDB, cfg))
	svc := analyticssvc.NewAnalyticsService(gormDB)

	_, err := svc.UserMetric(ctx, connect.NewRequest(&analyticsifacev1.UserMetricRequest{
		MetricTypes: []analyticsifacev1.MetricType{
			analyticsifacev1.MetricType_METRIC_TYPE_ORDER,
			analyticsifacev1.MetricType_METRIC_TYPE_STOCK,
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestUserMetric_StockSortRejected asserts the second leg of the USER rule: a
// Sort whose field is STOCK is rejected with InvalidArgument even when STOCK is
// not in metric_types (forbidStock=true on the user dimension).
func TestUserMetric_StockSortRejected(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ctx := servicetest.OwnerCtx(context.Background(), servicetest.EnsureOwner(t, gormDB, cfg))
	svc := analyticssvc.NewAnalyticsService(gormDB)

	_, err := svc.UserMetric(ctx, connect.NewRequest(&analyticsifacev1.UserMetricRequest{
		MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
		Sort: &analyticsifacev1.Sort{
			Field: &analyticsifacev1.Sort_Stock{
				Stock: analyticsifacev1.StockMetricField_STOCK_METRIC_FIELD_READY,
			},
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

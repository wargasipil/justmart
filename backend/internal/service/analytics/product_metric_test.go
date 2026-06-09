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

// TestProductMetric_EmptyCatalog is the happy path with no products seeded: the
// page is empty (no candidate ids), total is 0, and the metric blocks are nil
// (the handler short-circuits when there are no ids).
func TestProductMetric_EmptyCatalog(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ctx := servicetest.OwnerCtx(context.Background(), servicetest.EnsureOwner(t, gormDB, cfg))
	svc := analyticssvc.NewAnalyticsService(gormDB)

	now := time.Now()
	resp, err := svc.ProductMetric(ctx, connect.NewRequest(&analyticsifacev1.ProductMetricRequest{
		MetricTypes: []analyticsifacev1.MetricType{
			analyticsifacev1.MetricType_METRIC_TYPE_ORDER,
			analyticsifacev1.MetricType_METRIC_TYPE_STOCK,
		},
		Filter: &analyticsifacev1.Filter{
			FromUnix: now.AddDate(0, 0, -30).Unix(),
			ToUnix:   now.Unix(),
		},
		Limit:  25,
		Offset: 0,
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.ProductIds)
	require.Equal(t, int32(0), resp.Msg.Total)
	// No ids -> handler returns before populating metric blocks.
	require.Nil(t, resp.Msg.Order)
	require.Nil(t, resp.Msg.Stock)
}

// TestProductMetric_SortReferencesUnrequestedMetric asserts the validateSort
// precondition: a Sort on STOCK while metric_types only requests ORDER is
// rejected with InvalidArgument.
func TestProductMetric_SortReferencesUnrequestedMetric(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ctx := servicetest.OwnerCtx(context.Background(), servicetest.EnsureOwner(t, gormDB, cfg))
	svc := analyticssvc.NewAnalyticsService(gormDB)

	_, err := svc.ProductMetric(ctx, connect.NewRequest(&analyticsifacev1.ProductMetricRequest{
		MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
		Sort: &analyticsifacev1.Sort{
			Direction: analyticsifacev1.SortDirection_SORT_DIRECTION_DESC,
			Field: &analyticsifacev1.Sort_Stock{
				Stock: analyticsifacev1.StockMetricField_STOCK_METRIC_FIELD_READY,
			},
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestProductMetric_DuplicateMetricTypes asserts the validateMetricTypes
// precondition: a duplicated metric type is rejected with InvalidArgument.
func TestProductMetric_DuplicateMetricTypes(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ctx := servicetest.OwnerCtx(context.Background(), servicetest.EnsureOwner(t, gormDB, cfg))
	svc := analyticssvc.NewAnalyticsService(gormDB)

	_, err := svc.ProductMetric(ctx, connect.NewRequest(&analyticsifacev1.ProductMetricRequest{
		MetricTypes: []analyticsifacev1.MetricType{
			analyticsifacev1.MetricType_METRIC_TYPE_ORDER,
			analyticsifacev1.MetricType_METRIC_TYPE_ORDER,
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

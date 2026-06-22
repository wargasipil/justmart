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

// TestProductMetric_FilterByCashierNarrowsOrder proves the cashier filter narrows
// per-product revenue + COGS. Two cashiers each sell the same product; filtering
// to one shows only that cashier's contribution.
func TestProductMetric_FilterByCashierNarrowsOrder(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	svc := analyticssvc.NewAnalyticsService(gormDB)

	var wh model.Warehouse
	require.NoError(t, gormDB.Where("is_default").First(&wh).Error)
	cashierB := model.User{Email: "pm-b@x.test", Name: "B", PasswordHash: "x", Role: "CASHIER", Active: true}
	require.NoError(t, gormDB.Create(&cashierB).Error)

	prod := model.Product{SKU: "pm-cogs", Name: "Paracetamol", Unit: "tab", UnitPrice: 2000, Active: true}
	require.NoError(t, gormDB.Create(&prod).Error)
	batch := model.Batch{ProductID: prod.ID, BatchNumber: "B-1", CostPrice: 500,
		ExpiryDate: time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC), ReceivedAt: time.Now()}
	require.NoError(t, gormDB.Create(&batch).Error)

	seedSale := func(cashierID string, lineTotal int64, qty int32) {
		now := time.Now()
		whID := wh.ID
		sale := model.Sale{CashierUserID: cashierID, WarehouseID: &whID,
			Subtotal: lineTotal, Total: lineTotal, PaidAmount: lineTotal,
			Status: common.SaleStatusCompleted, CompletedAt: &now}
		require.NoError(t, gormDB.Create(&sale).Error)
		item := model.SaleItem{SaleID: sale.ID, ProductID: prod.ID, Qty: qty,
			BaseQty: qty, UnitFactor: 1, LineTotal: lineTotal}
		require.NoError(t, gormDB.Create(&item).Error)
		mv := model.StockMovement{BatchID: batch.ID, Qty: -qty, Type: "SALE",
			SaleItemID: &item.ID, UserID: cashierID, WarehouseID: wh.ID}
		require.NoError(t, gormDB.Create(&mv).Error)
	}
	seedSale(ownerID, 6000, 3)   // owner: revenue 6000, COGS 1500
	seedSale(cashierB.ID, 4000, 2) // B: revenue 4000, COGS 1000

	now := time.Now()
	get := func(cashierID string) *analyticsifacev1.OrderItem {
		resp, err := svc.ProductMetric(ctx, connect.NewRequest(&analyticsifacev1.ProductMetricRequest{
			MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
			Filter: &analyticsifacev1.Filter{
				FromUnix:      now.AddDate(0, 0, -1).Unix(),
				ToUnix:        now.AddDate(0, 0, 1).Unix(),
				CashierUserId: cashierID,
			},
			Limit: 25,
		}))
		require.NoError(t, err)
		require.NotNil(t, resp.Msg.Order)
		o := resp.Msg.Order.Data[prod.ID]
		require.NotNil(t, o)
		return o
	}

	all := get("")
	require.Equal(t, int64(10000), all.Terjual)
	require.Equal(t, int64(2500), all.Hpp)

	ownerOnly := get(ownerID)
	require.Equal(t, int64(6000), ownerOnly.Terjual)
	require.Equal(t, int64(1500), ownerOnly.Hpp, "COGS must be filtered by cashier too")
	require.Equal(t, int64(4500), ownerOnly.Profit)
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

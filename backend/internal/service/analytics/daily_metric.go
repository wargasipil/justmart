package analytics

import (
	"context"
	"time"

	"connectrpc.com/connect"

	analyticsifacev1 "github.com/justmart/backend/gen/analytics_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/service/common"
)

func (a *AnalyticsService) DailyMetric(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.DailyMetricRequest],
) (*connect.Response[analyticsifacev1.DailyMetricResponse], error) {
	if err := validateMetricTypes(req.Msg.MetricTypes, analyticsifacev1.MetricType_METRIC_TYPE_UNSPECIFIED, ""); err != nil {
		return nil, err
	}
	if err := validateSort(req.Msg.Sort, req.Msg.MetricTypes, false); err != nil {
		return nil, err
	}

	from, to := analyticsDateRange(req.Msg.Filter)
	gran := truncFmt(req.Msg.Granularity)
	wantOrder := containsMetric(req.Msg.MetricTypes, analyticsifacev1.MetricType_METRIC_TYPE_ORDER)
	wantStock := containsMetric(req.Msg.MetricTypes, analyticsifacev1.MetricType_METRIC_TYPE_STOCK)

	caller, perr := auth.MustPrincipal(ctx)
	if perr != nil {
		return nil, perr
	}
	warehouseID, werr := common.ResolveWarehouse(ctx, a.db, caller)
	if werr != nil {
		return nil, connect.NewError(connect.CodeInternal, werr)
	}

	buckets := enumerateBuckets(from, to, gran)
	if len(buckets) == 0 {
		return connect.NewResponse(&analyticsifacev1.DailyMetricResponse{}), nil
	}
	bucketKeys := make([]string, 0, len(buckets))
	for _, b := range buckets {
		bucketKeys = append(bucketKeys, dayBucketKey(b, gran))
	}
	var err error

	orderByKey := map[string]*analyticsifacev1.OrderItem{}
	if wantOrder {
		orderByKey, err = a.dailyOrderMetric(ctx, from, to, gran, warehouseID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	stockByKey := map[string]*analyticsifacev1.StockItem{}
	if wantStock {
		stockByKey, err = a.dailyStockMetric(ctx, warehouseID, buckets, bucketKeys, gran)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	dayOrder := sortDailyKeys(bucketKeys, req.Msg.Sort, orderByKey, stockByKey)

	out := &analyticsifacev1.DailyMetricResponse{Days: dayOrder}
	if wantOrder {
		out.Order = &analyticsifacev1.MetricOrder{Data: ensureOrderForKeys(orderByKey, dayOrder)}
	}
	if wantStock {
		out.Stock = &analyticsifacev1.MetricStock{Data: ensureStockForKeys(stockByKey, dayOrder)}
	}
	return connect.NewResponse(out), nil
}

func (a *AnalyticsService) dailyOrderMetric(ctx context.Context, from, to time.Time, gran, warehouseID string) (map[string]*analyticsifacev1.OrderItem, error) {
	type row struct {
		Day     string `gorm:"column:day"`
		Terjual int64
		Hpp     int64
	}
	var rows []row
	err := a.db.WithContext(ctx).Raw(`
		SELECT `+common.DayKeyExpr(a.db, "s.completed_at")+` AS day,
		       COALESCE(SUM(si.line_total), 0) AS terjual,
		       COALESCE(SUM(c.cogs), 0)        AS hpp
		FROM sales s
		JOIN sale_items si ON si.sale_id = s.id
		LEFT JOIN (
		  SELECT sm.sale_item_id, SUM(ABS(sm.qty) * COALESCE(b.cost_price, 0)) AS cogs
		  FROM stock_movements sm
		  JOIN batches b ON b.id = sm.batch_id
		  WHERE sm.type = 'SALE' AND sm.sale_item_id IS NOT NULL
		  GROUP BY sm.sale_item_id
		) c ON c.sale_item_id = si.id
		WHERE s.status = ? AND s.warehouse_id = ?
		  AND s.completed_at >= ? AND s.completed_at < ?
		GROUP BY day
	`, common.SaleStatusCompleted, warehouseID, from, to).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]*analyticsifacev1.OrderItem{}
	for _, r := range rows {
		t, perr := time.ParseInLocation("2006-01-02", r.Day, time.Local)
		if perr != nil {
			continue
		}
		key := dayBucketKey(bucketStart(t, gran), gran)
		item := out[key]
		if item == nil {
			item = &analyticsifacev1.OrderItem{}
			out[key] = item
		}
		item.Terjual += r.Terjual
		item.Hpp += r.Hpp
		item.Profit = item.Terjual - item.Hpp
	}
	return out, nil
}

func (a *AnalyticsService) dailyStockMetric(
	ctx context.Context,
	warehouseID string,
	buckets []time.Time,
	bucketKeys []string,
	gran string,
) (map[string]*analyticsifacev1.StockItem, error) {
	var ongoing int64
	if err := a.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(ordered_qty - received_qty), 0)
		FROM purchase_order_items poi
		JOIN purchase_orders po ON po.id = poi.purchase_order_id
		WHERE po.status NOT IN ('VOIDED', 'CLOSED', 'RECEIVED')
	`).Scan(&ongoing).Error; err != nil {
		return nil, err
	}

	out := map[string]*analyticsifacev1.StockItem{}
	for i, b := range buckets {
		var ready int64
		var bucketEnd time.Time
		switch gran {
		case "month":
			bucketEnd = b.AddDate(0, 1, 0)
		case "week":
			bucketEnd = b.AddDate(0, 0, 7)
		default:
			bucketEnd = b.AddDate(0, 0, 1)
		}
		if err := a.db.WithContext(ctx).Raw(`
			SELECT COALESCE(SUM(qty), 0)
			FROM stock_movements
			WHERE warehouse_id = ? AND created_at < ?
		`, warehouseID, bucketEnd).Scan(&ready).Error; err != nil {
			return nil, err
		}
		out[bucketKeys[i]] = &analyticsifacev1.StockItem{
			Ready:   ready,
			Ongoing: ongoing,
		}
	}
	return out, nil
}

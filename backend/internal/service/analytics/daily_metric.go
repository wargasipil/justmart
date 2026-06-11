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
	out := map[string]*analyticsifacev1.OrderItem{}
	accum := func(day string, addTerjual, addHpp int64) {
		t, perr := time.ParseInLocation("2006-01-02", day, time.Local)
		if perr != nil {
			return
		}
		key := dayBucketKey(bucketStart(t, gran), gran)
		item := out[key]
		if item == nil {
			item = &analyticsifacev1.OrderItem{}
			out[key] = item
		}
		item.Terjual += addTerjual
		item.Hpp += addHpp
		item.Profit = item.Terjual - item.Hpp
	}

	// Revenue per sale. Uses the sale-level total (NOT SUM(sale_items.line_total))
	// so terjual INCLUDES the biaya_jasa service fee (and any cart discount),
	// matching GetSalesSummary / GetTodaySnapshot / UserMetric — all of which sum
	// sale.total. Summing per-sale also avoids multiplying a sale-level fee by the
	// line count. (ProductMetric keeps item-only revenue — a sale-level fee isn't
	// attributable to a single product; documented carve-out there.)
	type revRow struct {
		Day     string `gorm:"column:day"`
		Terjual int64
	}
	var revRows []revRow
	if err := a.db.WithContext(ctx).Raw(`
		SELECT `+common.DayKeyExpr(a.db, "s.completed_at")+` AS day,
		       COALESCE(SUM(s.total), 0) AS terjual
		FROM sales s
		WHERE s.status = ? AND s.warehouse_id = ?
		  AND s.completed_at >= ? AND s.completed_at < ?
		GROUP BY day
	`, common.SaleStatusCompleted, warehouseID, from, to).Scan(&revRows).Error; err != nil {
		return nil, err
	}
	for _, r := range revRows {
		accum(r.Day, r.Terjual, 0)
	}

	// COGS per day from SALE stock movements (|qty| x batch cost), joined by
	// sale_item — correct under multi-unit / multi-batch lines.
	type cogsRow struct {
		Day string `gorm:"column:day"`
		Hpp int64
	}
	var cogsRows []cogsRow
	if err := a.db.WithContext(ctx).Raw(`
		SELECT `+common.DayKeyExpr(a.db, "s.completed_at")+` AS day,
		       COALESCE(SUM(c.cogs), 0) AS hpp
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
	`, common.SaleStatusCompleted, warehouseID, from, to).Scan(&cogsRows).Error; err != nil {
		return nil, err
	}
	for _, r := range cogsRows {
		accum(r.Day, 0, r.Hpp)
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

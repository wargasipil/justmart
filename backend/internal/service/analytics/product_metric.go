package analytics

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	analyticsifacev1 "github.com/justmart/backend/gen/analytics_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/service/common"
)

func (a *AnalyticsService) ProductMetric(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.ProductMetricRequest],
) (*connect.Response[analyticsifacev1.ProductMetricResponse], error) {
	if err := validateMetricTypes(req.Msg.MetricTypes, analyticsifacev1.MetricType_METRIC_TYPE_UNSPECIFIED, ""); err != nil {
		return nil, err
	}
	if err := validateSort(req.Msg.Sort, req.Msg.MetricTypes, false); err != nil {
		return nil, err
	}

	from, to := analyticsDateRange(req.Msg.Filter)
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
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

	ids, total, err := a.productPageIDs(ctx, from, to, warehouseID, req.Msg.Sort, wantOrder, wantStock, limit, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := &analyticsifacev1.ProductMetricResponse{
		ProductIds: ids,
		Total:      int32(total),
	}
	if len(ids) == 0 {
		return connect.NewResponse(out), nil
	}

	if wantOrder {
		om, err := a.productOrderForIDs(ctx, from, to, warehouseID, ids)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		out.Order = &analyticsifacev1.MetricOrder{Data: ensureOrderForKeys(om, ids)}
	}
	if wantStock {
		sm, err := a.productStockForIDs(ctx, warehouseID, ids)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		out.Stock = &analyticsifacev1.MetricStock{Data: ensureStockForKeys(sm, ids)}
	}
	return connect.NewResponse(out), nil
}

// productPageIDs builds the universe of candidate product ids, then sorts +
// paginates.
func (a *AnalyticsService) productPageIDs(
	ctx context.Context,
	from, to time.Time,
	warehouseID string,
	sort *analyticsifacev1.Sort,
	wantOrder, wantStock bool,
	limit, offset int,
) ([]string, int64, error) {
	var orderClause string
	if sort != nil {
		dir := sortDirSQL(sort.Direction)
		switch f := sort.Field.(type) {
		case *analyticsifacev1.Sort_Order:
			orderClause = fmt.Sprintf("ORDER BY %s %s NULLS LAST, m.name ASC", orderMetricColumn(f.Order), dir)
		case *analyticsifacev1.Sort_Stock:
			orderClause = fmt.Sprintf("ORDER BY %s %s NULLS LAST, m.name ASC", stockMetricColumn(f.Stock), dir)
		}
	}
	if orderClause == "" {
		orderClause = "ORDER BY m.name ASC"
	}

	combined := fmt.Sprintf(`
		WITH order_agg AS (
		  SELECT si.product_id,
		         SUM(si.line_total) AS terjual,
		         COALESCE(SUM(c.cogs), 0) AS hpp,
		         COALESCE(SUM(si.base_qty), 0) AS base_qty,
		         COALESCE(%s, 0) AS last_order_unix
		  FROM sale_items si
		  JOIN sales s ON s.id = si.sale_id
		  LEFT JOIN (
		    SELECT sm.sale_item_id, SUM(ABS(sm.qty) * COALESCE(b.cost_price, 0)) AS cogs
		    FROM stock_movements sm
		    JOIN batches b ON b.id = sm.batch_id
		    WHERE sm.type = 'SALE' AND sm.sale_item_id IS NOT NULL
		    GROUP BY sm.sale_item_id
		  ) c ON c.sale_item_id = si.id
		  WHERE s.status = ? AND s.warehouse_id = ?
		    AND s.completed_at >= ? AND s.completed_at < ?
		  GROUP BY si.product_id
		),
		stock_agg AS (
		  SELECT b.product_id,
		         COALESCE(SUM(sm.qty), 0) AS ready
		  FROM batches b
		  LEFT JOIN stock_movements sm ON sm.batch_id = b.id AND sm.warehouse_id = ?
		  GROUP BY b.product_id
		),
		ongoing_agg AS (
		  SELECT poi.product_id, COALESCE(SUM(poi.ordered_qty - poi.received_qty), 0) AS ongoing
		  FROM purchase_order_items poi
		  JOIN purchase_orders po ON po.id = poi.purchase_order_id
		  WHERE po.status NOT IN ('VOIDED', 'CLOSED', 'RECEIVED')
		  GROUP BY poi.product_id
		),
		restock_agg AS (
		  SELECT b.product_id,
		         %s AS last_restock_unix
		  FROM batches b
		  JOIN stock_movements sm ON sm.batch_id = b.id
		  WHERE sm.warehouse_id = ? AND sm.qty > 0
		  GROUP BY b.product_id
		),
		expiring_agg AS (
		  SELECT b.product_id,
		         COALESCE(SUM(sm.qty), 0) AS expiring
		  FROM batches b
		  JOIN stock_movements sm ON sm.batch_id = b.id AND sm.warehouse_id = ?
		  WHERE b.expiry_date >= CURRENT_DATE
		    AND b.expiry_date < %s
		  GROUP BY b.product_id
		),
		combined AS (
		  SELECT m.id, m.name,
		         COALESCE(o.terjual, 0) AS terjual,
		         COALESCE(o.hpp, 0) AS hpp,
		         (COALESCE(o.terjual, 0) - COALESCE(o.hpp, 0)) AS profit,
		         COALESCE(o.last_order_unix, 0) AS last_order_unix,
		         CASE WHEN ? <= 0 THEN 0
		              ELSE (COALESCE(o.base_qty, 0) + ?/2) / ? END AS avg_sold,
		         COALESCE(s.ready, 0) AS ready,
		         COALESCE(g.ongoing, 0) AS ongoing,
		         COALESCE(r.last_restock_unix, 0) AS last_restock_unix,
		         COALESCE(e.expiring, 0) AS expiring
		  FROM products m
		  LEFT JOIN order_agg o    ON o.product_id = m.id
		  LEFT JOIN stock_agg s    ON s.product_id = m.id
		  LEFT JOIN ongoing_agg g  ON g.product_id = m.id
		  LEFT JOIN restock_agg r  ON r.product_id = m.id
		  LEFT JOIN expiring_agg e ON e.product_id = m.id
		  WHERE m.active = true
		)
	`, common.EpochExpr(a.db, "MAX(s.completed_at)"), common.EpochExpr(a.db, "MAX(b.received_at)"), common.DateAddNowDays(a.db, 30))

	days := int64(to.Sub(from) / (24 * time.Hour))
	if days < 1 {
		days = 1
	}

	var total int64
	if err := a.db.WithContext(ctx).Raw(
		combined+`SELECT COUNT(*) FROM combined`,
		common.SaleStatusCompleted, warehouseID, from, to,
		warehouseID, warehouseID, warehouseID,
		days, days, days,
	).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	type idRow struct{ ID string }
	var rows []idRow
	err := a.db.WithContext(ctx).Raw(
		combined+`SELECT id FROM combined m `+orderClause+` LIMIT ? OFFSET ?`,
		common.SaleStatusCompleted, warehouseID, from, to,
		warehouseID, warehouseID, warehouseID,
		days, days, days,
		limit, offset,
	).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids, total, nil
}

func (a *AnalyticsService) productOrderForIDs(ctx context.Context, from, to time.Time, warehouseID string, ids []string) (map[string]*analyticsifacev1.OrderItem, error) {
	type row struct {
		ProductID     string `gorm:"column:product_id"`
		Terjual       int64
		Hpp           int64
		BaseQty       int64 `gorm:"column:base_qty"`
		LastOrderUnix int64 `gorm:"column:last_order_unix"`
	}
	var rows []row
	err := a.db.WithContext(ctx).Raw(`
		SELECT si.product_id,
		       COALESCE(SUM(si.line_total), 0) AS terjual,
		       COALESCE(SUM(c.cogs), 0) AS hpp,
		       COALESCE(SUM(si.base_qty), 0) AS base_qty,
		       COALESCE(`+common.EpochExpr(a.db, "MAX(s.completed_at)")+`, 0) AS last_order_unix
		FROM sale_items si
		JOIN sales s ON s.id = si.sale_id
		LEFT JOIN (
		  SELECT sm.sale_item_id, SUM(ABS(sm.qty) * COALESCE(b.cost_price, 0)) AS cogs
		  FROM stock_movements sm
		  JOIN batches b ON b.id = sm.batch_id
		  WHERE sm.type = 'SALE' AND sm.sale_item_id IS NOT NULL
		  GROUP BY sm.sale_item_id
		) c ON c.sale_item_id = si.id
		WHERE s.status = ? AND s.warehouse_id = ?
		  AND s.completed_at >= ? AND s.completed_at < ?
		  AND si.product_id IN ?
		GROUP BY si.product_id
	`, common.SaleStatusCompleted, warehouseID, from, to, ids).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	days := int64(to.Sub(from) / (24 * time.Hour))
	if days < 1 {
		days = 1
	}
	out := map[string]*analyticsifacev1.OrderItem{}
	for _, r := range rows {
		out[r.ProductID] = &analyticsifacev1.OrderItem{
			Terjual:       r.Terjual,
			Hpp:           r.Hpp,
			Profit:        r.Terjual - r.Hpp,
			LastOrderUnix: r.LastOrderUnix,
			AvgSold:       (r.BaseQty + days/2) / days, // rounded
		}
	}
	return out, nil
}

func (a *AnalyticsService) productStockForIDs(ctx context.Context, warehouseID string, ids []string) (map[string]*analyticsifacev1.StockItem, error) {
	type row struct {
		ProductID       string `gorm:"column:product_id"`
		Ready           int64
		Ongoing         int64
		LastRestockUnix int64 `gorm:"column:last_restock_unix"`
		Expiring        int64
	}
	var rows []row
	err := a.db.WithContext(ctx).Raw(fmt.Sprintf(`
		SELECT m.id AS product_id,
		       COALESCE(s.ready, 0)   AS ready,
		       COALESCE(g.ongoing, 0) AS ongoing,
		       COALESCE(r.last_restock_unix, 0) AS last_restock_unix,
		       COALESCE(e.expiring, 0) AS expiring
		FROM products m
		LEFT JOIN (
		  SELECT b.product_id, COALESCE(SUM(sm.qty), 0) AS ready
		  FROM batches b
		  LEFT JOIN stock_movements sm ON sm.batch_id = b.id AND sm.warehouse_id = ?
		  GROUP BY b.product_id
		) s ON s.product_id = m.id
		LEFT JOIN (
		  SELECT poi.product_id, COALESCE(SUM(poi.ordered_qty - poi.received_qty), 0) AS ongoing
		  FROM purchase_order_items poi
		  JOIN purchase_orders po ON po.id = poi.purchase_order_id
		  WHERE po.status NOT IN ('VOIDED', 'CLOSED', 'RECEIVED')
		  GROUP BY poi.product_id
		) g ON g.product_id = m.id
		LEFT JOIN (
		  SELECT b.product_id,
		         %s AS last_restock_unix
		  FROM batches b
		  JOIN stock_movements sm ON sm.batch_id = b.id
		  WHERE sm.warehouse_id = ? AND sm.qty > 0
		  GROUP BY b.product_id
		) r ON r.product_id = m.id
		LEFT JOIN (
		  SELECT b.product_id, COALESCE(SUM(sm.qty), 0) AS expiring
		  FROM batches b
		  JOIN stock_movements sm ON sm.batch_id = b.id AND sm.warehouse_id = ?
		  WHERE b.expiry_date >= CURRENT_DATE
		    AND b.expiry_date < %s
		  GROUP BY b.product_id
		) e ON e.product_id = m.id
		WHERE m.id IN ?
	`, common.EpochExpr(a.db, "MAX(b.received_at)"), common.DateAddNowDays(a.db, 30)), warehouseID, warehouseID, warehouseID, ids).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]*analyticsifacev1.StockItem{}
	for _, r := range rows {
		out[r.ProductID] = &analyticsifacev1.StockItem{
			Ready:           r.Ready,
			Ongoing:         r.Ongoing,
			LastRestockUnix: r.LastRestockUnix,
			Expiring:        r.Expiring,
		}
	}
	return out, nil
}

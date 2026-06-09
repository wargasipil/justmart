package analytics

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	analyticsifacev1 "github.com/justmart/backend/gen/analytics_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/service/common"
)

func (a *AnalyticsService) UserMetric(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.UserMetricRequest],
) (*connect.Response[analyticsifacev1.UserMetricResponse], error) {
	// USER dimension rejects STOCK in any form (metric_types or sort.field).
	if err := validateMetricTypes(req.Msg.MetricTypes,
		analyticsifacev1.MetricType_METRIC_TYPE_STOCK,
		"stock metric not supported on user dimension"); err != nil {
		return nil, err
	}
	if err := validateSort(req.Msg.Sort, req.Msg.MetricTypes, true); err != nil {
		return nil, err
	}

	from, to := analyticsDateRange(req.Msg.Filter)
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	wantOrder := containsMetric(req.Msg.MetricTypes, analyticsifacev1.MetricType_METRIC_TYPE_ORDER)

	caller, perr := auth.MustPrincipal(ctx)
	if perr != nil {
		return nil, perr
	}
	warehouseID, werr := common.ResolveWarehouse(ctx, a.db, caller)
	if werr != nil {
		return nil, connect.NewError(connect.CodeInternal, werr)
	}

	var orderClause string
	if req.Msg.Sort != nil {
		if f, ok := req.Msg.Sort.Field.(*analyticsifacev1.Sort_Order); ok {
			orderClause = fmt.Sprintf("ORDER BY %s %s NULLS LAST, u.name ASC",
				orderMetricColumn(f.Order), sortDirSQL(req.Msg.Sort.Direction))
		}
	}
	if orderClause == "" {
		orderClause = "ORDER BY u.name ASC"
	}

	combined := `
		WITH order_agg AS (
		  SELECT s.cashier_user_id AS user_id,
		         COALESCE(SUM(s.total), 0) AS terjual,
		         COALESCE(SUM(c.cogs), 0) AS hpp
		  FROM sales s
		  LEFT JOIN (
		    SELECT si.sale_id, SUM(ABS(sm.qty) * COALESCE(b.cost_price, 0)) AS cogs
		    FROM sale_items si
		    JOIN stock_movements sm ON sm.sale_item_id = si.id AND sm.type = 'SALE'
		    JOIN batches b ON b.id = sm.batch_id
		    GROUP BY si.sale_id
		  ) c ON c.sale_id = s.id
		  WHERE s.status = ? AND s.warehouse_id = ?
		    AND s.completed_at >= ? AND s.completed_at < ?
		  GROUP BY s.cashier_user_id
		),
		combined AS (
		  SELECT u.id, u.name,
		         COALESCE(o.terjual, 0) AS terjual,
		         COALESCE(o.hpp, 0) AS hpp,
		         (COALESCE(o.terjual, 0) - COALESCE(o.hpp, 0)) AS profit
		  FROM users u
		  JOIN order_agg o ON o.user_id = u.id
		  WHERE u.active = true
		)
	`

	var total int64
	if err := a.db.WithContext(ctx).Raw(
		combined+`SELECT COUNT(*) FROM combined`,
		common.SaleStatusCompleted, warehouseID, from, to,
	).Scan(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	type idRow struct{ ID string }
	var rows []idRow
	if err := a.db.WithContext(ctx).Raw(
		combined+`SELECT id FROM combined u `+orderClause+` LIMIT ? OFFSET ?`,
		common.SaleStatusCompleted, warehouseID, from, to, limit, offset,
	).Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}

	out := &analyticsifacev1.UserMetricResponse{
		UserIds: ids,
		Total:   int32(total),
	}
	if len(ids) == 0 {
		return connect.NewResponse(out), nil
	}

	if wantOrder {
		type orow struct {
			UserID  string `gorm:"column:user_id"`
			Terjual int64
			Hpp     int64
		}
		var orows []orow
		err := a.db.WithContext(ctx).Raw(`
			SELECT s.cashier_user_id AS user_id,
			       COALESCE(SUM(s.total), 0) AS terjual,
			       COALESCE(SUM(c.cogs), 0) AS hpp
			FROM sales s
			LEFT JOIN (
			  SELECT si.sale_id, SUM(ABS(sm.qty) * COALESCE(b.cost_price, 0)) AS cogs
			  FROM sale_items si
			  JOIN stock_movements sm ON sm.sale_item_id = si.id AND sm.type = 'SALE'
			  JOIN batches b ON b.id = sm.batch_id
			  GROUP BY si.sale_id
			) c ON c.sale_id = s.id
			WHERE s.status = ? AND s.warehouse_id = ?
			  AND s.completed_at >= ? AND s.completed_at < ?
			  AND s.cashier_user_id IN ?
			GROUP BY s.cashier_user_id
		`, common.SaleStatusCompleted, warehouseID, from, to, ids).Scan(&orows).Error
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		m := map[string]*analyticsifacev1.OrderItem{}
		for _, r := range orows {
			m[r.UserID] = &analyticsifacev1.OrderItem{
				Terjual: r.Terjual,
				Hpp:     r.Hpp,
				Profit:  r.Terjual - r.Hpp,
			}
		}
		out.Order = &analyticsifacev1.MetricOrder{Data: ensureOrderForKeys(m, ids)}
	}
	return connect.NewResponse(out), nil
}

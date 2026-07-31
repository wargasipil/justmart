package product

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ListLowStock returns one page of the active products whose ready_stock in the
// caller's active warehouse is at or below the configured low-stock threshold.
//
// `total` is the FULL match count, not the page length — the TopBar bell badge
// reads it. It used to be len(products) under a hard Limit(100), so a shop with
// more than 100 low items showed a badge reading exactly "100".
func (s *ProductService) ListLowStock(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListLowStockRequest],
) (*connect.Response[inventoryifacev1.ListLowStockResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	threshold, err := common.GetLowStockThreshold(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)

	// One filter closure feeds both the count and the page, so the two can't drift.
	applyFilters := func(q *gorm.DB) *gorm.DB {
		return q.Table("products AS m").
			Joins("LEFT JOIN batches AS b ON b.product_id = m.id").
			Joins("LEFT JOIN stock_movements AS sm ON sm.batch_id = b.id AND sm.warehouse_id = ?", warehouseID).
			Where("m.active = ?", true).
			Group("m.id").
			Having("COALESCE(SUM(sm.qty), 0) <= ?", threshold)
	}
	// GROUP BY + HAVING can't be counted directly — Count would return one row
	// per group. Wrap it as a derived table and count the groups.
	var total int64
	if err := s.db.WithContext(ctx).
		Table("(?) AS t", applyFilters(s.db.WithContext(ctx)).Select("m.id")).
		Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	type lowRow struct {
		model.Product
		Ready int64 `gorm:"column:ready"`
	}
	var rows []lowRow
	// m.id breaks the (ready, name) tie so paging is deterministic.
	if err := applyFilters(s.db.WithContext(ctx)).
		Select("m.*, COALESCE(SUM(sm.qty), 0) AS ready").
		Order("ready ASC, m.name ASC, m.id ASC").
		Offset(offset).Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*inventoryifacev1.Product, 0, len(rows))
	for i := range rows {
		p := productToProto(&rows[i].Product)
		p.ReadyStock = rows[i].Ready
		out = append(out, p)
	}
	if err := s.attachUnits(ctx, out); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.ListLowStockResponse{
		Products:  out,
		Threshold: threshold,
		Total:     int32(total),
	}), nil
}

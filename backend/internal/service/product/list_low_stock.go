package product

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ListLowStock returns active products whose ready_stock in the caller's active
// warehouse is at or below the configured low-stock threshold. Capped at 100.
func (s *ProductService) ListLowStock(
	ctx context.Context,
	_ *connect.Request[inventoryifacev1.ListLowStockRequest],
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

	type lowRow struct {
		model.Product
		Ready int64 `gorm:"column:ready"`
	}
	var rows []lowRow
	if err := s.db.WithContext(ctx).
		Table("products AS m").
		Select("m.*, COALESCE(SUM(sm.qty), 0) AS ready").
		Joins("LEFT JOIN batches AS b ON b.product_id = m.id").
		Joins("LEFT JOIN stock_movements AS sm ON sm.batch_id = b.id AND sm.warehouse_id = ?", warehouseID).
		Where("m.active = ?", true).
		Group("m.id").
		Having("COALESCE(SUM(sm.qty), 0) <= ?", threshold).
		Order("ready ASC, m.name ASC").
		Limit(100).
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
		Total:     int32(len(out)),
	}), nil
}

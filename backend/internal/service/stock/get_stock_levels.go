package stock

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/service/common"
)

func (s *StockService) GetStockLevels(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetStockLevelsRequest],
) (*connect.Response[inventoryifacev1.GetStockLevelsResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}

	// Stock is scoped to the caller's active warehouse — the warehouse filter
	// lives in the JOIN so batches with no stock there still appear (qty 0).
	q := s.db.WithContext(ctx).
		Table("batches b").
		Select("b.id AS batch_id, b.product_id, "+common.DayKeyExpr(s.db, "b.expiry_date")+" AS expiry_date, COALESCE(SUM(m.qty), 0) AS current_quantity").
		Joins("LEFT JOIN stock_movements m ON m.batch_id = b.id AND m.warehouse_id = ?", warehouseID).
		Group("b.id").
		Order("b.expiry_date ASC")

	if req.Msg.ProductId != "" {
		q = q.Where("b.product_id = ?", req.Msg.ProductId)
	}

	type row struct {
		BatchID         string `gorm:"column:batch_id"`
		ProductID       string `gorm:"column:product_id"`
		ExpiryDate      string `gorm:"column:expiry_date"`
		CurrentQuantity int64  `gorm:"column:current_quantity"`
	}
	var rows []row
	if err := q.Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*inventoryifacev1.StockLevel, 0, len(rows))
	for _, r := range rows {
		out = append(out, &inventoryifacev1.StockLevel{
			BatchId:         r.BatchID,
			ProductId:       r.ProductID,
			ExpiryDate:      r.ExpiryDate,
			CurrentQuantity: r.CurrentQuantity,
		})
	}
	return connect.NewResponse(&inventoryifacev1.GetStockLevelsResponse{Levels: out}), nil
}

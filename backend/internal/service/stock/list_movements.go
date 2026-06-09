package stock

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *StockService) ListMovements(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListMovementsRequest],
) (*connect.Response[inventoryifacev1.ListMovementsResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		// Scope to the caller's active warehouse so the Mutasi page + the
		// product-detail Movements tab + the Mutasi CSV export all show only
		// this warehouse's ledger.
		q = q.Where("warehouse_id = ?", warehouseID)
		if req.Msg.BatchId != "" {
			q = q.Where("batch_id = ?", req.Msg.BatchId)
		}
		if req.Msg.ProductId != "" {
			q = q.Where("batch_id IN (?)",
				s.db.Table("batches").Select("id").Where("product_id = ?", req.Msg.ProductId))
		}
		if t := movementTypeToString(req.Msg.Type); t != "" {
			q = q.Where("type = ?", t)
		}
		if query := strings.TrimSpace(req.Msg.Query); query != "" {
			pattern := "%" + query + "%"
			q = q.Where("batch_id IN (?)",
				s.db.Table("batches b").
					Select("b.id").
					Joins("JOIN products m ON m.id = b.product_id").
					Where("b.batch_number "+common.LikeOp(s.db)+" ? OR m.name "+common.LikeOp(s.db)+" ? OR m.sku "+common.LikeOp(s.db)+" ?", pattern, pattern, pattern))
		}
		if req.Msg.FromUnix > 0 {
			q = q.Where("created_at >= ?", time.Unix(req.Msg.FromUnix, 0))
		}
		if req.Msg.ToUnix > 0 {
			q = q.Where("created_at < ?", time.Unix(req.Msg.ToUnix, 0))
		}
		return q
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.StockMovement{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.StockMovement
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.StockMovement{})).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.StockMovement, 0, len(rows))
	for _, r := range rows {
		out = append(out, movementToProto(&r))
	}
	return connect.NewResponse(&inventoryifacev1.ListMovementsResponse{
		Movements: out,
		Total:     int32(total),
	}), nil
}

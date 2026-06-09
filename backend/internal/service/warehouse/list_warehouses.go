package warehouse

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *WarehouseService) ListWarehouses(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.ListWarehousesRequest],
) (*connect.Response[warehouseifacev1.ListWarehousesResponse], error) {
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	query := strings.TrimSpace(req.Msg.Query)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		if !req.Msg.IncludeInactive {
			q = q.Where("active = ?", true)
		}
		if query != "" {
			like := "%" + query + "%"
			q = q.Where("code "+common.LikeOp(q)+" ? OR name "+common.LikeOp(q)+" ?", like, like)
		}
		return q
	}

	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Warehouse{})).
		Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var rows []model.Warehouse
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Warehouse{})).
		Order("code ASC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*warehouseifacev1.Warehouse, 0, len(rows))
	for i := range rows {
		out = append(out, warehouseToProto(&rows[i]))
	}
	return connect.NewResponse(&warehouseifacev1.ListWarehousesResponse{
		Warehouses: out,
		Total:      int32(total),
	}), nil
}

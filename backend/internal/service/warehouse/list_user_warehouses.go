package warehouse

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *WarehouseService) ListUserWarehouses(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.ListUserWarehousesRequest],
) (*connect.Response[warehouseifacev1.ListUserWarehousesResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	target := req.Msg.UserId
	if target == "" {
		target = caller.UserID
	}
	if target != caller.UserID && caller.Role != "OWNER" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("can only list own memberships"))
	}

	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)

	// Page the JOIN, not the memberships alone. `query` filters warehouses, so
	// paging memberships separately would make the two response arrays describe
	// different sets and make `total` ignore the search — the caller pairs them
	// by warehouse id, and a membership whose warehouse got filtered out is a
	// dangling row.
	applyFilters := func(q *gorm.DB) *gorm.DB {
		q = q.Model(&model.UserWarehouse{}).
			Joins("JOIN warehouses w ON w.id = user_warehouses.warehouse_id").
			Where("user_warehouses.user_id = ? AND w.active = ?", target, true)
		if query := strings.TrimSpace(req.Msg.Query); query != "" {
			like := "%" + query + "%"
			q = q.Where("w.code "+common.LikeOp(q)+" ? OR w.name "+common.LikeOp(q)+" ?", like, like)
		}
		return q
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	type joined struct {
		model.Warehouse
		IsDefault bool `gorm:"column:is_default"`
	}
	var rows []joined
	if err := applyFilters(s.db.WithContext(ctx)).
		Select("w.*, user_warehouses.is_default AS is_default").
		Order("w.code ASC, w.id ASC").
		Offset(offset).Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	outMems := make([]*warehouseifacev1.UserWarehouseMembership, 0, len(rows))
	outWhs := make([]*warehouseifacev1.Warehouse, 0, len(rows))
	for i := range rows {
		outMems = append(outMems, &warehouseifacev1.UserWarehouseMembership{
			UserId:      target,
			WarehouseId: rows[i].Warehouse.ID,
			IsDefault:   rows[i].IsDefault,
		})
		outWhs = append(outWhs, warehouseToProto(&rows[i].Warehouse))
	}
	return connect.NewResponse(&warehouseifacev1.ListUserWarehousesResponse{
		Memberships: outMems,
		Warehouses:  outWhs,
		Total:       int32(total),
	}), nil
}

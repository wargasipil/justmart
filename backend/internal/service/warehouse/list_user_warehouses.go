package warehouse

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

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

	var mems []model.UserWarehouse
	if err := s.db.WithContext(ctx).Where("user_id = ?", target).Find(&mems).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(mems) == 0 {
		return connect.NewResponse(&warehouseifacev1.ListUserWarehousesResponse{}), nil
	}
	ids := make([]string, 0, len(mems))
	for _, m := range mems {
		ids = append(ids, m.WarehouseID)
	}
	var whs []model.Warehouse
	q := s.db.WithContext(ctx).Where("id IN ? AND active = ?", ids, true)
	if query := strings.TrimSpace(req.Msg.Query); query != "" {
		like := "%" + query + "%"
		q = q.Where("code "+common.LikeOp(q)+" ? OR name "+common.LikeOp(q)+" ?", like, like)
	}
	if err := q.Order("code ASC").Find(&whs).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	outMems := make([]*warehouseifacev1.UserWarehouseMembership, 0, len(mems))
	for _, m := range mems {
		outMems = append(outMems, &warehouseifacev1.UserWarehouseMembership{
			UserId:      m.UserID,
			WarehouseId: m.WarehouseID,
			IsDefault:   m.IsDefault,
		})
	}
	outWhs := make([]*warehouseifacev1.Warehouse, 0, len(whs))
	for i := range whs {
		outWhs = append(outWhs, warehouseToProto(&whs[i]))
	}
	return connect.NewResponse(&warehouseifacev1.ListUserWarehousesResponse{
		Memberships: outMems,
		Warehouses:  outWhs,
	}), nil
}

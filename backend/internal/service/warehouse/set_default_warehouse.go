package warehouse

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *WarehouseService) SetDefaultWarehouse(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.SetDefaultWarehouseRequest],
) (*connect.Response[warehouseifacev1.SetDefaultWarehouseResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	target := req.Msg.UserId
	if target == "" {
		target = caller.UserID
	}
	if target != caller.UserID && caller.Role != "OWNER" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("can only set own default"))
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var mem model.UserWarehouse
		if err := tx.Where("user_id = ? AND warehouse_id = ?", target, req.Msg.WarehouseId).
			First(&mem).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodePermissionDenied,
					errors.New("user has no access to that warehouse"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := tx.Model(&model.UserWarehouse{}).
			Where("user_id = ?", target).Update("is_default", false).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := tx.Model(&model.UserWarehouse{}).
			Where("user_id = ? AND warehouse_id = ?", target, req.Msg.WarehouseId).
			Update("is_default", true).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	return connect.NewResponse(&warehouseifacev1.SetDefaultWarehouseResponse{
		Membership: &warehouseifacev1.UserWarehouseMembership{
			UserId:      target,
			WarehouseId: req.Msg.WarehouseId,
			IsDefault:   true,
		},
	}), nil
}

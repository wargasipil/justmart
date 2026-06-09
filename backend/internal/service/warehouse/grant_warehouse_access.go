package warehouse

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *WarehouseService) GrantWarehouseAccess(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.GrantWarehouseAccessRequest],
) (*connect.Response[warehouseifacev1.GrantWarehouseAccessResponse], error) {
	if req.Msg.UserId == "" || req.Msg.WarehouseId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id and warehouse_id required"))
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		mem := model.UserWarehouse{
			UserID:      req.Msg.UserId,
			WarehouseID: req.Msg.WarehouseId,
			IsDefault:   req.Msg.IsDefault,
		}
		if err := tx.Save(&mem).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if req.Msg.IsDefault {
			if err := tx.Model(&model.UserWarehouse{}).
				Where("user_id = ? AND warehouse_id <> ?", req.Msg.UserId, req.Msg.WarehouseId).
				Update("is_default", false).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	return connect.NewResponse(&warehouseifacev1.GrantWarehouseAccessResponse{
		Membership: &warehouseifacev1.UserWarehouseMembership{
			UserId:      req.Msg.UserId,
			WarehouseId: req.Msg.WarehouseId,
			IsDefault:   req.Msg.IsDefault,
		},
	}), nil
}

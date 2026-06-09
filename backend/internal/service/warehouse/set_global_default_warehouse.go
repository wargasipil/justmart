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

// SetGlobalDefaultWarehouse promotes the given warehouse to the company-wide
// default. The partial unique index on `is_default` enforces "only one default
// at a time"; we clear the old default + set the new one in one tx.
func (s *WarehouseService) SetGlobalDefaultWarehouse(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.SetGlobalDefaultWarehouseRequest],
) (*connect.Response[warehouseifacev1.SetGlobalDefaultWarehouseResponse], error) {
	if req.Msg.WarehouseId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("warehouse_id required"))
	}
	row, err := s.load(ctx, req.Msg.WarehouseId)
	if err != nil {
		return nil, err
	}
	if !row.Active {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("cannot promote an archived warehouse"))
	}
	if row.IsDefault {
		return connect.NewResponse(&warehouseifacev1.SetGlobalDefaultWarehouseResponse{
			Warehouse: warehouseToProto(row),
		}), nil
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Warehouse{}).Where("is_default").
			Update("is_default", false).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := tx.Model(&model.Warehouse{}).Where("id = ?", row.ID).
			Update("is_default", true).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	row.IsDefault = true
	return connect.NewResponse(&warehouseifacev1.SetGlobalDefaultWarehouseResponse{
		Warehouse: warehouseToProto(row),
	}), nil
}

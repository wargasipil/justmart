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

func (s *WarehouseService) CreateWarehouse(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.CreateWarehouseRequest],
) (*connect.Response[warehouseifacev1.CreateWarehouseResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	code := strings.TrimSpace(strings.ToUpper(req.Msg.Code))
	name := strings.TrimSpace(req.Msg.Name)
	if code == "" || name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("code and name required"))
	}
	row := model.Warehouse{
		Code:    code,
		Name:    name,
		Address: strings.TrimSpace(req.Msg.Address),
		Phone:   strings.TrimSpace(req.Msg.Phone),
		Active:  true,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		// Auto-grant the creator access so they can use it immediately.
		return tx.Save(&model.UserWarehouse{
			UserID:      caller.UserID,
			WarehouseID: row.ID,
		}).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	return connect.NewResponse(&warehouseifacev1.CreateWarehouseResponse{Warehouse: warehouseToProto(&row)}), nil
}

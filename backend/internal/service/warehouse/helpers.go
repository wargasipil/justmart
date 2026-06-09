package warehouse

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (s *WarehouseService) load(ctx context.Context, id string) (*model.Warehouse, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var row model.Warehouse
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("warehouse not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &row, nil
}

func warehouseToProto(w *model.Warehouse) *warehouseifacev1.Warehouse {
	return &warehouseifacev1.Warehouse{
		Id:        w.ID,
		Code:      w.Code,
		Name:      w.Name,
		Address:   w.Address,
		Phone:     w.Phone,
		IsDefault: w.IsDefault,
		Active:    w.Active,
		CreatedAt: w.CreatedAt.Unix(),
	}
}

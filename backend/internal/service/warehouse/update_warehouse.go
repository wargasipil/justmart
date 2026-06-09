package warehouse

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
)

func (s *WarehouseService) UpdateWarehouse(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.UpdateWarehouseRequest],
) (*connect.Response[warehouseifacev1.UpdateWarehouseResponse], error) {
	row, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name required"))
	}
	updates := map[string]any{
		"name":    name,
		"address": strings.TrimSpace(req.Msg.Address),
		"phone":   strings.TrimSpace(req.Msg.Phone),
	}
	if err := s.db.WithContext(ctx).Model(row).Updates(updates).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	row, err = s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&warehouseifacev1.UpdateWarehouseResponse{Warehouse: warehouseToProto(row)}), nil
}

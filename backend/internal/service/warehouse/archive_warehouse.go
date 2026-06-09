package warehouse

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
)

func (s *WarehouseService) ArchiveWarehouse(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.ArchiveWarehouseRequest],
) (*connect.Response[warehouseifacev1.ArchiveWarehouseResponse], error) {
	row, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if row.IsDefault {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("cannot archive the default warehouse"))
	}
	if err := s.db.WithContext(ctx).Model(row).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	row.Active = false
	return connect.NewResponse(&warehouseifacev1.ArchiveWarehouseResponse{Warehouse: warehouseToProto(row)}), nil
}

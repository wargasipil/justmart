package warehouse

import (
	"context"

	"connectrpc.com/connect"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
)

// GetWarehouse returns a single warehouse by id. Drives the WarehouseDetail
// page; readable by any authenticated role.
func (s *WarehouseService) GetWarehouse(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.GetWarehouseRequest],
) (*connect.Response[warehouseifacev1.GetWarehouseResponse], error) {
	row, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&warehouseifacev1.GetWarehouseResponse{
		Warehouse: warehouseToProto(row),
	}), nil
}

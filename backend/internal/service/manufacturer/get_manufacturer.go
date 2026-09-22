package manufacturer

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func (s *ManufacturerService) GetManufacturer(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetManufacturerRequest],
) (*connect.Response[inventoryifacev1.GetManufacturerResponse], error) {
	m, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.GetManufacturerResponse{
		Manufacturer: manufacturerToProto(m),
	}), nil
}

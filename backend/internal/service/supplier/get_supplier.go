package supplier

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func (s *SupplierService) GetSupplier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetSupplierRequest],
) (*connect.Response[inventoryifacev1.GetSupplierResponse], error) {
	sup, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.GetSupplierResponse{Supplier: supplierToProto(sup)}), nil
}

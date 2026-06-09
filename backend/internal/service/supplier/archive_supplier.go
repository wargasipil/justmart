package supplier

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func (s *SupplierService) ArchiveSupplier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ArchiveSupplierRequest],
) (*connect.Response[inventoryifacev1.ArchiveSupplierResponse], error) {
	sup, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(sup).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	sup.Active = false
	return connect.NewResponse(&inventoryifacev1.ArchiveSupplierResponse{Supplier: supplierToProto(sup)}), nil
}

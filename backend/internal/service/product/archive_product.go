package product

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func (s *ProductService) ArchiveProduct(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ArchiveProductRequest],
) (*connect.Response[inventoryifacev1.ArchiveProductResponse], error) {
	med, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(med).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	med.Active = false
	return connect.NewResponse(&inventoryifacev1.ArchiveProductResponse{Product: productToProto(med)}), nil
}

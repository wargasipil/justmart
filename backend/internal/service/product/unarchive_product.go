package product

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func (s *ProductService) UnarchiveProduct(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UnarchiveProductRequest],
) (*connect.Response[inventoryifacev1.UnarchiveProductResponse], error) {
	med, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(med).Update("active", true).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	med.Active = true
	return connect.NewResponse(&inventoryifacev1.UnarchiveProductResponse{Product: productToProto(med)}), nil
}

package product

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (s *ProductService) ListProductPrices(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductPricesRequest],
) (*connect.Response[inventoryifacev1.ListProductPricesResponse], error) {
	if req.Msg.ProductId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_id required"))
	}
	var rows []model.ProductPrice
	err := s.db.WithContext(ctx).
		Where("product_id = ?", req.Msg.ProductId).
		Order("effective_from DESC").
		Find(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.ProductPrice, 0, len(rows))
	for _, r := range rows {
		out = append(out, productPriceToProto(&r))
	}
	return connect.NewResponse(&inventoryifacev1.ListProductPricesResponse{Prices: out}), nil
}

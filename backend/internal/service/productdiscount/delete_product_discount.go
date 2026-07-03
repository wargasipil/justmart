package productdiscount

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (s *ProductDiscountService) DeleteProductDiscount(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.DeleteProductDiscountRequest],
) (*connect.Response[inventoryifacev1.DeleteProductDiscountResponse], error) {
	d, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Where("id = ?", d.ID).Delete(&model.ProductDiscount{}).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&inventoryifacev1.DeleteProductDiscountResponse{}), nil
}

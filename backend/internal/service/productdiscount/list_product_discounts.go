package productdiscount

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// ListProductDiscounts returns all discounts for one product (newest first). A
// product has only a handful, so this is unpaginated.
func (s *ProductDiscountService) ListProductDiscounts(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductDiscountsRequest],
) (*connect.Response[inventoryifacev1.ListProductDiscountsResponse], error) {
	productID := strings.TrimSpace(req.Msg.ProductId)
	if productID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_id required"))
	}
	var rows []model.ProductDiscount
	if err := s.db.WithContext(ctx).Where("product_id = ?", productID).
		Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.ProductDiscount, len(rows))
	for i := range rows {
		out[i] = toProto(&rows[i])
	}
	return connect.NewResponse(&inventoryifacev1.ListProductDiscountsResponse{Discounts: out}), nil
}

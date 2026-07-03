package productdiscount

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (s *ProductDiscountService) CreateProductDiscount(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.CreateProductDiscountRequest],
) (*connect.Response[inventoryifacev1.CreateProductDiscountResponse], error) {
	msg := req.Msg
	if err := productExists(s.db.WithContext(ctx), msg.ProductId); err != nil {
		return nil, err
	}
	normType, err := validateDiscount(msg.DiscountType, msg.Value, msg.MinQty)
	if err != nil {
		return nil, err
	}
	expires, err := parseExpiry(msg.ExpiresAt)
	if err != nil {
		return nil, err
	}
	unitID, unitName, unitFactor, err := resolveMinQtyUnit(ctx, s.db, msg.ProductId, msg.MinQtyUnitId)
	if err != nil {
		return nil, err
	}
	d := &model.ProductDiscount{
		ProductID:        msg.ProductId,
		DiscountType:     normType,
		PerItem:          msg.PerItem,
		Value:            msg.Value,
		MinQty:           msg.MinQty,
		MinQtyUnitID:     unitID,
		MinQtyUnitName:   unitName,
		MinQtyUnitFactor: unitFactor,
		ExpiresAt:        expires,
	}
	if err := s.db.WithContext(ctx).Create(d).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&inventoryifacev1.CreateProductDiscountResponse{Discount: toProto(d)}), nil
}

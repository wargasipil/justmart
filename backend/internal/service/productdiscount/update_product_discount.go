package productdiscount

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

// UpdateProductDiscount edits a discount's mode/value/rule/expiry. product_id is
// immutable.
func (s *ProductDiscountService) UpdateProductDiscount(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UpdateProductDiscountRequest],
) (*connect.Response[inventoryifacev1.UpdateProductDiscountResponse], error) {
	msg := req.Msg
	d, err := s.load(ctx, msg.Id)
	if err != nil {
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
	unitID, unitName, unitFactor, err := resolveMinQtyUnit(ctx, s.db, d.ProductID, msg.MinQtyUnitId)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(d).Updates(map[string]any{
		"discount_type":       normType,
		"per_item":            msg.PerItem,
		"value":               msg.Value,
		"min_qty":             msg.MinQty,
		"min_qty_unit_id":     unitID,
		"min_qty_unit_name":   unitName,
		"min_qty_unit_factor": unitFactor,
		"expires_at":          expires,
	}).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&inventoryifacev1.UpdateProductDiscountResponse{Discount: toProto(d)}), nil
}

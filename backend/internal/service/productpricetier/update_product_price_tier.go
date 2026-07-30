package productpricetier

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

// UpdateProductPriceTier edits a tier's unit, threshold and price. product_id is
// immutable — a tier belongs to the product it was created under.
func (s *ProductPriceTierService) UpdateProductPriceTier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UpdateProductPriceTierRequest],
) (*connect.Response[inventoryifacev1.UpdateProductPriceTierResponse], error) {
	msg := req.Msg
	t, err := s.load(ctx, msg.Id)
	if err != nil {
		return nil, err
	}
	if err := validateTier(msg.MinQty, msg.Price); err != nil {
		return nil, err
	}
	unit, err := resolveTierUnit(ctx, s.db, t.ProductID, msg.ProductUnitId)
	if err != nil {
		return nil, err
	}
	// Exclude our own row so re-saving an unchanged tier isn't a conflict.
	if err := assertTierFree(s.db.WithContext(ctx), unit.ID, msg.MinQty, t.ID); err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(t).Updates(map[string]any{
		"product_unit_id": unit.ID,
		"unit_name":       unit.Name,
		"unit_factor":     unit.Factor,
		"min_qty":         msg.MinQty,
		"price":           msg.Price,
	}).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&inventoryifacev1.UpdateProductPriceTierResponse{Tier: toProto(t)}), nil
}

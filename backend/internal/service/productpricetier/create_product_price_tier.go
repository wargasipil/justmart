package productpricetier

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *ProductPriceTierService) CreateProductPriceTier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.CreateProductPriceTierRequest],
) (*connect.Response[inventoryifacev1.CreateProductPriceTierResponse], error) {
	msg := req.Msg
	if err := productExists(s.db.WithContext(ctx), msg.ProductId); err != nil {
		return nil, err
	}
	if err := validateTier(msg.MinQty, msg.Price); err != nil {
		return nil, err
	}
	unit, err := resolveTierUnit(ctx, s.db, msg.ProductId, msg.ProductUnitId)
	if err != nil {
		return nil, err
	}
	if err := assertTierFree(s.db.WithContext(ctx), unit.ID, msg.MinQty, ""); err != nil {
		return nil, err
	}
	t := &model.ProductPriceTier{
		ProductID:     msg.ProductId,
		ProductUnitID: unit.ID,
		UnitName:      unit.Name,
		UnitFactor:    unit.Factor,
		MinQty:        msg.MinQty,
		Price:         msg.Price,
	}
	if err := s.db.WithContext(ctx).Create(t).Error; err != nil {
		// The unique index backstops a race with the pre-check above.
		return nil, common.TokenError(connect.CodeAlreadyExists, "product_price_tier.tier_taken")
	}
	return connect.NewResponse(&inventoryifacev1.CreateProductPriceTierResponse{Tier: toProto(t)}), nil
}

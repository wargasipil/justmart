package productpricetier

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// DeleteProductPriceTier hard-deletes a tier (mirrors DeleteProductDiscount).
// A DRAFT cart line priced by this tier keeps its snapshot until the next qty
// change, at which point it reverts to the normal price — same as a deleted
// product discount or an edited sell_price.
func (s *ProductPriceTierService) DeleteProductPriceTier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.DeleteProductPriceTierRequest],
) (*connect.Response[inventoryifacev1.DeleteProductPriceTierResponse], error) {
	t, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Where("id = ?", t.ID).Delete(&model.ProductPriceTier{}).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&inventoryifacev1.DeleteProductPriceTierResponse{}), nil
}

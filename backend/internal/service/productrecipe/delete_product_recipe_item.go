package productrecipe

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// DeleteProductRecipeItem removes an ingredient line. Hard delete, matching the
// productdiscount / productpricetier domains: a recipe line carries no history
// worth preserving (completed sales already recorded what they consumed as
// stock_movements, which are immutable and unaffected).
func (s *ProductRecipeService) DeleteProductRecipeItem(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.DeleteProductRecipeItemRequest],
) (*connect.Response[inventoryifacev1.DeleteProductRecipeItemResponse], error) {
	it, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).
		Delete(&model.ProductRecipeItem{}, "id = ?", it.ID).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&inventoryifacev1.DeleteProductRecipeItemResponse{}), nil
}

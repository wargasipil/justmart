package productrecipe

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

// UpdateProductRecipeItem edits a line's quantity and note.
//
// The parent and the component are IMMUTABLE: swapping the component in place
// would silently redirect what a sale deducts while keeping the line's identity,
// which reads in the UI as an edit but is really a different ingredient. Delete
// the line and add the new one instead — one more click, and the intent is
// visible.
func (s *ProductRecipeService) UpdateProductRecipeItem(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UpdateProductRecipeItemRequest],
) (*connect.Response[inventoryifacev1.UpdateProductRecipeItemResponse], error) {
	it, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := validateQtyBase(req.Msg.QtyBase); err != nil {
		return nil, err
	}
	it.QtyBase = req.Msg.QtyBase
	it.Note = strings.TrimSpace(req.Msg.Note)
	if err := s.db.WithContext(ctx).
		Model(it).
		Select("qty_base", "note", "updated_at").
		Updates(it).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&inventoryifacev1.UpdateProductRecipeItemResponse{Item: toProto(it)}), nil
}

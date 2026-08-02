package productrecipe

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// CreateProductRecipeItem adds one ingredient line to a COMPOSITE product's
// recipe. Every precondition is checked before the insert so the caller gets a
// specific token rather than a raw constraint violation; the unique index and
// the CHECKs backstop the races.
func (s *ProductRecipeService) CreateProductRecipeItem(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.CreateProductRecipeItemRequest],
) (*connect.Response[inventoryifacev1.CreateProductRecipeItemResponse], error) {
	msg := req.Msg
	db := s.db.WithContext(ctx)

	if err := assertCompositeParent(db, msg.ProductId); err != nil {
		return nil, err
	}
	if err := assertUsableComponent(db, msg.ProductId, msg.ComponentProductId); err != nil {
		return nil, err
	}
	if err := validateQtyBase(msg.QtyBase); err != nil {
		return nil, err
	}
	if err := assertComponentFree(db, msg.ProductId, msg.ComponentProductId, ""); err != nil {
		return nil, err
	}

	it := &model.ProductRecipeItem{
		ProductID:          msg.ProductId,
		ComponentProductID: msg.ComponentProductId,
		QtyBase:            msg.QtyBase,
		Note:               strings.TrimSpace(msg.Note),
	}
	if err := db.Create(it).Error; err != nil {
		return nil, common.TokenError(connect.CodeAlreadyExists, "product_recipe.component_taken")
	}
	return connect.NewResponse(&inventoryifacev1.CreateProductRecipeItemResponse{Item: toProto(it)}), nil
}

package productrecipe

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *ProductRecipeService) load(ctx context.Context, id string) (*model.ProductRecipeItem, error) {
	if strings.TrimSpace(id) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var it model.ProductRecipeItem
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&it).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("recipe item %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &it, nil
}

func toProto(it *model.ProductRecipeItem) *inventoryifacev1.ProductRecipeItem {
	return &inventoryifacev1.ProductRecipeItem{
		Id:                 it.ID,
		ProductId:          it.ProductID,
		ComponentProductId: it.ComponentProductID,
		QtyBase:            it.QtyBase,
		Note:               it.Note,
		CreatedAt:          it.CreatedAt.Unix(),
	}
}

// assertCompositeParent requires the recipe's owner to exist and be COMPOSITE.
// A recipe on a STOCKED product would be inert (its sale consumes its own
// batches and never looks at the recipe), so silently accepting one would mean
// an owner filling in ingredients that never get deducted.
func assertCompositeParent(db *gorm.DB, productID string) error {
	if strings.TrimSpace(productID) == "" {
		return common.TokenError(connect.CodeInvalidArgument, "product_recipe.product_missing")
	}
	var p model.Product
	err := db.Select("id", "product_kind").Where("id = ?", productID).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.TokenError(connect.CodeFailedPrecondition, "product_recipe.product_missing")
	}
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if common.NormalizeProductKind(p.Kind) != common.ProductKindComposite {
		return common.TokenError(connect.CodeFailedPrecondition, "product_recipe.parent_not_composite")
	}
	return nil
}

// assertUsableComponent requires the ingredient to exist, be active, and be
// STOCKED.
//
// The STOCKED requirement is what makes nested recipes unrepresentable, and with
// them every recipe cycle: a component can never itself be COMPOSITE, so the
// explosion at CompleteSale is always exactly one level deep and needs no cycle
// detection or depth bound. Multi-level recipes (a sauce made from ingredients,
// used in a dish) are a documented non-goal for this version — the workaround is
// to list the sauce's ingredients directly on the dish.
//
// SERVICE components are rejected for the same reason a recipe on a STOCKED
// parent is: they hold no stock, so the line could never deduct anything.
func assertUsableComponent(db *gorm.DB, parentID, componentID string) error {
	componentID = strings.TrimSpace(componentID)
	if componentID == "" {
		return common.TokenError(connect.CodeInvalidArgument, "product_recipe.component_missing")
	}
	if componentID == parentID {
		return common.TokenError(connect.CodeInvalidArgument, "product_recipe.component_self")
	}
	var c model.Product
	err := db.Select("id", "product_kind", "active").Where("id = ?", componentID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.TokenError(connect.CodeFailedPrecondition, "product_recipe.component_missing")
	}
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if !c.Active {
		return common.TokenError(connect.CodeFailedPrecondition, "product_recipe.component_archived")
	}
	if common.NormalizeProductKind(c.Kind) != common.ProductKindStocked {
		return common.TokenError(connect.CodeFailedPrecondition, "product_recipe.component_not_stocked")
	}
	return nil
}

func validateQtyBase(qty int64) error {
	if qty <= 0 {
		return common.TokenError(connect.CodeInvalidArgument, "product_recipe.qty_invalid")
	}
	return nil
}

// assertComponentFree pre-checks the (product_id, component_product_id) unique
// index so the caller gets a specific token instead of a raw constraint
// violation. One line per ingredient: to use more of it, raise qty_base.
func assertComponentFree(db *gorm.DB, productID, componentID, excludeID string) error {
	q := db.Model(&model.ProductRecipeItem{}).
		Where("product_id = ? AND component_product_id = ?", productID, componentID)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if n > 0 {
		return common.TokenError(connect.CodeAlreadyExists, "product_recipe.component_taken")
	}
	return nil
}

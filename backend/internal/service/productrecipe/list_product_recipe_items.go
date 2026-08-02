package productrecipe

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ListProductRecipeItems returns one page of a COMPOSITE product's recipe lines,
// oldest first (the order they were entered, which is how a cook reads a recipe),
// with a stable id tiebreaker so paging can't drop or duplicate a line.
//
// Each line is enriched with its component's name/sku/base-unit and current
// on-hand in the CALLER'S ACTIVE WAREHOUSE — the same scoping every other stock
// read uses, so the recipe card agrees with the Batches tab next to it. Both
// enrichments are batch-loaded (no N+1).
//
// `buildable` answers the question the card actually exists to answer: how many
// portions can I make right now = min over lines of floor(component_ready /
// qty_base). It is computed over the WHOLE recipe, not the page — a page-local
// minimum would report a larger number than the kitchen can actually cook.
func (s *ProductRecipeService) ListProductRecipeItems(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductRecipeItemsRequest],
) (*connect.Response[inventoryifacev1.ListProductRecipeItemsResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	productID := strings.TrimSpace(req.Msg.ProductId)
	if productID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_id required"))
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)

	applyFilters := func(q *gorm.DB) *gorm.DB {
		return q.Model(&model.ProductRecipeItem{}).Where("product_id = ?", productID)
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.ProductRecipeItem
	if err := applyFilters(s.db.WithContext(ctx)).
		Order("created_at ASC, id ASC").
		Offset(offset).Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*inventoryifacev1.ProductRecipeItem, len(rows))
	for i := range rows {
		out[i] = toProto(&rows[i])
	}
	if err := s.enrichComponents(ctx, caller, out); err != nil {
		return nil, err
	}

	buildable, err := s.buildablePortions(ctx, caller, productID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.ListProductRecipeItemsResponse{
		Items:     out,
		Total:     int32(total),
		Buildable: buildable,
	}), nil
}

// enrichComponents batch-fills the display fields (name, sku, base unit) and the
// component's on-hand in the active warehouse for a page of recipe lines.
func (s *ProductRecipeService) enrichComponents(
	ctx context.Context,
	caller auth.Principal,
	items []*inventoryifacev1.ProductRecipeItem,
) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ComponentProductId)
	}

	var prods []model.Product
	if err := s.db.WithContext(ctx).
		Select("id", "sku", "name", "unit").
		Where("id IN ?", ids).Find(&prods).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	byID := make(map[string]*model.Product, len(prods))
	for i := range prods {
		byID[prods[i].ID] = &prods[i]
	}

	ready, err := common.ReadyStockByProduct(ctx, s.db, caller, ids)
	if err != nil {
		return err
	}
	for _, it := range items {
		if p, ok := byID[it.ComponentProductId]; ok {
			it.ComponentName = p.Name
			it.ComponentSku = p.SKU
			it.ComponentUnit = p.Unit
		}
		it.ComponentReady = ready[it.ComponentProductId]
	}
	return nil
}

// buildablePortions is min over the FULL recipe of floor(component_ready /
// qty_base) — how many base units of the parent the current component stock can
// produce in the active warehouse. Returns -1 for a product with no recipe: no
// line means nothing bounds it, which is a different fact from "0 portions" and
// the UI renders it differently (an empty recipe, not an out-of-stock one).
func (s *ProductRecipeService) buildablePortions(
	ctx context.Context,
	caller auth.Principal,
	productID string,
) (int64, error) {
	var rows []model.ProductRecipeItem
	if err := s.db.WithContext(ctx).
		Where("product_id = ?", productID).Find(&rows).Error; err != nil {
		return 0, connect.NewError(connect.CodeInternal, err)
	}
	if len(rows) == 0 {
		return -1, nil
	}
	ids := make([]string, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].ComponentProductID)
	}
	ready, err := common.ReadyStockByProduct(ctx, s.db, caller, ids)
	if err != nil {
		return 0, err
	}
	return common.BuildablePortions(rows, ready), nil
}

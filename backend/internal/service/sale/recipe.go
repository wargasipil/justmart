package sale

import (
	"connectrpc.com/connect"
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// consumption is one "take N base units of product P, on behalf of cart line L"
// instruction produced by resolving a cart against the product kinds.
//
// Keeping the SALE line id on every row is what makes recipes free for the rest
// of the system: COGS is computed from SALE stock_movements joined by
// sale_item_id (never from the sold product's own batches), so an exploded
// ingredient movement lands in exactly the same aggregation as an ordinary one.
// A menu item's food cost is therefore real without analytics knowing recipes
// exist.
type consumption struct {
	SaleItemID string
	ProductID  string
	NeedBase   int32
}

// resolveConsumption expands a cart into what it actually takes out of stock:
//
//	STOCKED   -> itself, base_qty (the original behaviour, unchanged)
//	COMPOSITE -> each recipe component, base_qty x qty_base
//	SERVICE   -> nothing
//
// The explosion is exactly one level deep and needs no cycle detection: a recipe
// component is required to be STOCKED at write time (see productrecipe), so a
// component can never itself carry a recipe.
//
// A COMPOSITE with an empty recipe resolves to nothing and completes fine. That
// is the honest reading — the owner has declared it assembles from nothing yet —
// and the alternative (blocking the sale) would strand a shop mid-service over a
// catalog gap. The product list surfaces it as 0 buildable so it is visible.
func resolveConsumption(tx *gorm.DB, items []model.SaleItem) ([]consumption, error) {
	if len(items) == 0 {
		return nil, nil
	}

	productIDs := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for i := range items {
		if _, ok := seen[items[i].ProductID]; ok {
			continue
		}
		seen[items[i].ProductID] = struct{}{}
		productIDs = append(productIDs, items[i].ProductID)
	}

	var prods []model.Product
	if err := tx.Select("id", "product_kind").Where("id IN ?", productIDs).Find(&prods).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	kindByID := make(map[string]string, len(prods))
	for i := range prods {
		kindByID[prods[i].ID] = common.NormalizeProductKind(prods[i].Kind)
	}

	// Recipes for the composites in this cart, in one query.
	compositeIDs := make([]string, 0, len(productIDs))
	for _, id := range productIDs {
		if kindByID[id] == common.ProductKindComposite {
			compositeIDs = append(compositeIDs, id)
		}
	}
	recipeByParent := map[string][]model.ProductRecipeItem{}
	if len(compositeIDs) > 0 {
		var lines []model.ProductRecipeItem
		if err := tx.Where("product_id IN ?", compositeIDs).Find(&lines).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for i := range lines {
			recipeByParent[lines[i].ProductID] = append(recipeByParent[lines[i].ProductID], lines[i])
		}
	}

	out := make([]consumption, 0, len(items))
	for i := range items {
		item := items[i]
		need := item.BaseQty
		if need <= 0 {
			need = item.Qty // back-compat for rows created before UOM
		}
		switch kindByID[item.ProductID] {
		case common.ProductKindService:
			continue
		case common.ProductKindComposite:
			for _, line := range recipeByParent[item.ProductID] {
				out = append(out, consumption{
					SaleItemID: item.ID,
					ProductID:  line.ComponentProductID,
					// int32 throughout, matching sale_items.base_qty and
					// stock_movements.qty; a portion count x a per-portion
					// quantity stays far inside that range for any real cart.
					NeedBase: need * int32(line.QtyBase),
				})
			}
		default: // STOCKED, and anything unrecognized (NormalizeProductKind maps it here)
			out = append(out, consumption{
				SaleItemID: item.ID,
				ProductID:  item.ProductID,
				NeedBase:   need,
			})
		}
	}
	return out, nil
}

// consumedProductIDs is the deduped set of products a resolved cart will draw
// stock from — the lots that must be locked FOR UPDATE before availability is
// read. For a composite cart these are the INGREDIENTS, not the menu items:
// locking the menu item's (nonexistent) lots would leave two concurrent sales
// free to oversell the same jar of sauce.
func consumedProductIDs(cons []consumption) []string {
	seen := make(map[string]struct{}, len(cons))
	ids := make([]string, 0, len(cons))
	for _, c := range cons {
		if _, ok := seen[c.ProductID]; ok {
			continue
		}
		seen[c.ProductID] = struct{}{}
		ids = append(ids, c.ProductID)
	}
	return ids
}

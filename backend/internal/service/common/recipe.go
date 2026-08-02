package common

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
)

// ReadyStockByProduct returns on-hand BASE units per product id in the caller's
// ACTIVE WAREHOUSE, as one grouped query (no N+1). Products with no batches or
// no movements are simply absent from the map — read it with the zero value.
//
// Shared because three surfaces need the same number on the same scoping: the
// product list/detail enrich, the recipe card's per-component stock, and the
// composite availability math. Duplicating the join is how the "ready" figure on
// one screen drifts from the one next to it.
func ReadyStockByProduct(
	ctx context.Context,
	db *gorm.DB,
	caller auth.Principal,
	productIDs []string,
) (map[string]int64, error) {
	out := map[string]int64{}
	if len(productIDs) == 0 {
		return out, nil
	}
	warehouseID, err := ResolveWarehouse(ctx, db, caller)
	if err != nil {
		return nil, err
	}
	type row struct {
		ProductID string `gorm:"column:product_id"`
		Qty       int64  `gorm:"column:qty"`
	}
	var rows []row
	if err := db.WithContext(ctx).
		Table("batches AS b").
		Select("b.product_id AS product_id, COALESCE(SUM(sm.qty), 0) AS qty").
		Joins("LEFT JOIN stock_movements sm ON sm.batch_id = b.id AND sm.warehouse_id = ?", warehouseID).
		Where("b.product_id IN ?", productIDs).
		Group("b.product_id").Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, r := range rows {
		out[r.ProductID] = r.Qty
	}
	return out, nil
}

// BuildablePortions is how many BASE units of a composite product the given
// component stock can produce: min over its recipe lines of
// floor(component_ready / qty_base).
//
// The minimum (not the sum, not the average) is the whole point — a dish is
// bounded by whichever ingredient runs out first, so one empty jar makes the
// portion count 0 no matter how much of everything else is in the store.
//
// Returns -1 for an EMPTY recipe. That is deliberately not 0: "this composite has
// no recipe yet" and "this composite is out of an ingredient" are different
// facts, and collapsing them would make an unconfigured menu item look
// out-of-stock instead of unconfigured. Callers that want a stock number treat
// -1 as "unbounded / unknown", never as a quantity.
func BuildablePortions(recipe []model.ProductRecipeItem, ready map[string]int64) int64 {
	if len(recipe) == 0 {
		return -1
	}
	var min int64 = -1
	for i := range recipe {
		per := recipe[i].QtyBase
		if per <= 0 {
			continue // guarded by a CHECK; skip rather than divide by zero
		}
		n := ready[recipe[i].ComponentProductID] / per
		if n < 0 {
			n = 0
		}
		if min < 0 || n < min {
			min = n
		}
	}
	if min < 0 {
		return -1
	}
	return min
}

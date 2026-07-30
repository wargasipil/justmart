package sale

import (
	"connectrpc.com/connect"
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/model"
)

// resolveTierPrice returns the best qualifying "grosir" (wholesale) tier for a
// line of `qty` units priced at `listPrice`.
//
// A tier qualifies when it belongs to THIS unit, qty >= min_qty, and its price is
// STRICTLY cheaper than the list price. The strictness matters: a mis-entered
// tier at or above the normal price must not "apply", because applying it would
// set tier_min_qty > 0 and suppress the automatic product discount while giving
// the customer no price benefit at all — strictly worse than having no tier.
//
// Among qualifying tiers the LOWEST price wins (ties broken by the lowest
// threshold, since that is the one the customer actually earned). Selection runs
// in Go rather than SQL ORDER BY/LIMIT so ties resolve identically on SQLite and
// Postgres.
func resolveTierPrice(tx *gorm.DB, unitID string, qty int32, listPrice int64) (int64, int32, bool, error) {
	if unitID == "" || qty <= 0 {
		return 0, 0, false, nil
	}
	var rows []model.ProductPriceTier
	if err := tx.Where("product_unit_id = ?", unitID).Find(&rows).Error; err != nil {
		return 0, 0, false, connect.NewError(connect.CodeInternal, err)
	}
	var bestPrice int64
	var bestMinQty int32
	found := false
	for i := range rows {
		t := rows[i]
		if t.MinQty <= 0 || qty < t.MinQty {
			continue // threshold not reached
		}
		if t.Price < 0 || t.Price >= listPrice {
			continue // no benefit — see the strictness note above
		}
		if !found || t.Price < bestPrice || (t.Price == bestPrice && t.MinQty < bestMinQty) {
			bestPrice, bestMinQty, found = t.Price, t.MinQty, true
		}
	}
	return bestPrice, bestMinQty, found, nil
}

// applyTierPrice sets a line's EFFECTIVE price (UnitPriceSnapshot) and grosir
// flag (TierMinQty) from its frozen list price and current qty.
//
// It is deliberately a pure function of (ListPriceSnapshot, Qty): it never reads
// UnitPriceSnapshot, so lowering the qty back below a threshold restores the
// normal price instead of ratcheting the grosir price in place. Call it from
// every path that changes a line's qty or unit — AddItem (both the create and
// the merge branch) and SetItemQuantity — always AFTER refreshing
// ListPriceSnapshot from the unit and BEFORE recomputeLine, which reads both
// fields this sets.
func applyTierPrice(tx *gorm.DB, item *model.SaleItem) error {
	// Legacy pre-UOM rows carry a NULL product_unit_id and predate the list-price
	// column; fall back to the effective price so they behave exactly as before.
	if item.ListPriceSnapshot <= 0 {
		item.ListPriceSnapshot = item.UnitPriceSnapshot
	}
	if item.ProductUnitID == nil {
		item.UnitPriceSnapshot = item.ListPriceSnapshot
		item.TierMinQty = 0
		return nil
	}
	price, minQty, found, err := resolveTierPrice(tx, *item.ProductUnitID, item.Qty, item.ListPriceSnapshot)
	if err != nil {
		return err
	}
	if found {
		item.UnitPriceSnapshot = price
		item.TierMinQty = minQty
		return nil
	}
	item.UnitPriceSnapshot = item.ListPriceSnapshot
	item.TierMinQty = 0
	return nil
}

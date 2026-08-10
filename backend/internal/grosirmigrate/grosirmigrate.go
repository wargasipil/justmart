// Package grosirmigrate converts qty-gated product discounts
// (`product_discounts`) into grosir wholesale price tiers
// (`product_price_tiers`). It backs the `justmart discount-to-grosir` CLI
// subcommand and is deliberately split from it so the mapping rules are unit
// testable on both engines.
//
// The two rules describe overlapping intent but are NOT interchangeable, so the
// conversion is planned first and every non-convertible row is reported with a
// reason rather than silently dropped or force-fitted:
//
//   - A discount subtracts an amount from the line; a tier REPLACES the unit
//     price. That maps cleanly only when the discount is expressible per unit —
//     a PERCENT rule, or a FIXED rule that already applies per item.
//   - A discount's min_qty compares in BASE units and is therefore earned across
//     units (3 box ×12 = 36 base, satisfied by 36 pcs too); a tier binds to its
//     own unit only. The converted tier keeps the discount's threshold unit, so
//     it stops being earned by an equivalent quantity of a different unit.
//   - A tier needs min_qty >= 2 (DB CHECK), so an always-on discount (min_qty
//     0/1) has no tier form at all.
package grosirmigrate

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// minTierQty mirrors productpricetier.minTierQty and the DB CHECK. Duplicated
// rather than imported because that constant is unexported and this package
// must not depend on a service package.
const minTierQty = 2

const (
	discountFixed   = "FIXED"
	discountPercent = "PERCENT"
)

// Options tunes which discounts are considered convertible.
type Options struct {
	// IncludeLineFixed converts FIXED discounts whose amount applies to the whole
	// LINE (per_item = false). Off by default because there is no exact per-unit
	// equivalent: the discount stays flat as qty grows, while a tier scales with
	// it. When on, the amount is divided by the threshold — exact at min_qty,
	// increasingly generous above it.
	IncludeLineFixed bool
}

// Item is one planned conversion (Convert = true) or one skipped discount
// (Convert = false, Reason set).
//
// The DiscountType/PerItem/DiscountValue/ExpiresAt fields describe the SOURCE
// rule. They exist because Apply deletes it: the CSV report is then the only
// record of what a converted tier used to be.
type Item struct {
	DiscountID    string
	DiscountType  string
	PerItem       bool
	DiscountValue int64
	ExpiresAt     string // YYYY-MM-DD, empty when the discount never expires
	ProductID     string
	ProductName   string
	UnitID        string
	UnitName      string
	UnitFactor    int64
	MinQty        int32
	// ListPrice is the unit's current sell_price; TierPrice is what the tier
	// would charge. Both minor units. Zero on a skipped item that never got far
	// enough to resolve them.
	ListPrice int64
	TierPrice int64
	Convert   bool
	Reason    string
}

// Plan is the full report: every product_discounts row, in a stable order, each
// marked convertible or skipped.
type Plan struct {
	Items []Item
}

// Convertible returns the items Apply would write.
func (p *Plan) Convertible() []Item { return p.filter(true) }

// Skipped returns the items Apply would leave untouched, each with a Reason.
func (p *Plan) Skipped() []Item { return p.filter(false) }

func (p *Plan) filter(convert bool) []Item {
	var out []Item
	for _, it := range p.Items {
		if it.Convert == convert {
			out = append(out, it)
		}
	}
	return out
}

// BuildPlan reads every discount and decides its fate. It writes nothing.
func BuildPlan(db *gorm.DB, opts Options) (*Plan, error) {
	var discounts []model.ProductDiscount
	// Stable order so a dry run and the subsequent --apply report identically,
	// and so two discounts colliding on the same (unit, threshold) always resolve
	// the same way.
	if err := db.Order("product_id, min_qty, id").Find(&discounts).Error; err != nil {
		return nil, fmt.Errorf("load product discounts: %w", err)
	}
	plan := &Plan{}
	if len(discounts) == 0 {
		return plan, nil
	}

	productIDs := make([]string, 0, len(discounts))
	seen := map[string]bool{}
	for _, d := range discounts {
		if !seen[d.ProductID] {
			seen[d.ProductID] = true
			productIDs = append(productIDs, d.ProductID)
		}
	}

	var products []model.Product
	if err := db.Where("id IN ?", productIDs).Find(&products).Error; err != nil {
		return nil, fmt.Errorf("load products: %w", err)
	}
	nameByProduct := make(map[string]string, len(products))
	for _, p := range products {
		nameByProduct[p.ID] = p.Name
	}

	// Inactive units are loaded too, so an inactive threshold unit gets a precise
	// reason instead of looking like a missing row.
	var units []model.ProductUnit
	if err := db.Where("product_id IN ?", productIDs).Find(&units).Error; err != nil {
		return nil, fmt.Errorf("load product units: %w", err)
	}
	unitByID := make(map[string]model.ProductUnit, len(units))
	baseByProduct := make(map[string]model.ProductUnit, len(productIDs))
	for _, u := range units {
		unitByID[u.ID] = u
		if u.IsBase {
			baseByProduct[u.ProductID] = u
		}
	}

	// (product_unit_id, min_qty) is uniquely indexed. Pre-check it so a collision
	// reports as a skip instead of aborting the whole Apply transaction; the
	// insert still backstops the race.
	var tiers []model.ProductPriceTier
	if err := db.Where("product_id IN ?", productIDs).Find(&tiers).Error; err != nil {
		return nil, fmt.Errorf("load price tiers: %w", err)
	}
	taken := make(map[string]bool, len(tiers))
	for _, t := range tiers {
		taken[tierKey(t.ProductUnitID, t.MinQty)] = true
	}

	today := time.Now().Format(common.DateLayout)
	for i := range discounts {
		it := planOne(discounts[i], opts, today, nameByProduct, unitByID, baseByProduct, taken)
		if it.Convert {
			// Claim the slot so a second discount mapping to the same unit +
			// threshold is skipped rather than failing the insert.
			taken[tierKey(it.UnitID, it.MinQty)] = true
		}
		plan.Items = append(plan.Items, it)
	}
	return plan, nil
}

func planOne(
	d model.ProductDiscount,
	opts Options,
	today string,
	nameByProduct map[string]string,
	unitByID map[string]model.ProductUnit,
	baseByProduct map[string]model.ProductUnit,
	taken map[string]bool,
) Item {
	it := Item{
		DiscountID:    d.ID,
		DiscountType:  d.DiscountType,
		PerItem:       d.PerItem,
		DiscountValue: d.Value,
		ProductID:     d.ProductID,
		ProductName:   nameByProduct[d.ProductID],
		MinQty:        d.MinQty,
	}
	if it.DiscountType == "" {
		it.DiscountType = discountFixed
	}
	if d.ExpiresAt != nil {
		it.ExpiresAt = d.ExpiresAt.Format(common.DateLayout)
	}
	skip := func(reason string) Item {
		it.Convert = false
		it.Reason = reason
		return it
	}

	if d.MinQty < minTierQty {
		return skip(fmt.Sprintf("not qty-gated (min_qty %d, a tier needs >= %d)", d.MinQty, minTierQty))
	}
	if d.ExpiresAt != nil && d.ExpiresAt.Format(common.DateLayout) < today {
		return skip("already expired on " + d.ExpiresAt.Format(common.DateLayout))
	}

	unit, ok := resolveUnit(d, unitByID, baseByProduct)
	if !ok {
		if d.MinQtyUnitID != nil && *d.MinQtyUnitID != "" {
			return skip("threshold unit " + *d.MinQtyUnitID + " no longer exists")
		}
		return skip("product has no base unit")
	}
	it.UnitID, it.UnitName, it.UnitFactor = unit.ID, unit.Name, unit.Factor
	if it.UnitFactor < 1 {
		it.UnitFactor = 1
	}
	if !unit.Active {
		return skip("unit " + unit.Name + " is archived")
	}
	if !unit.Sellable {
		// A tier only ever applies to a sale line, so a non-sellable unit's tier
		// could never fire.
		return skip("unit " + unit.Name + " is not sellable")
	}
	it.ListPrice = unit.SellPrice
	if unit.SellPrice <= 0 {
		return skip("unit " + unit.Name + " has no sell price to discount from")
	}
	if taken[tierKey(unit.ID, d.MinQty)] {
		return skip(fmt.Sprintf("a tier already exists for %s >= %d", unit.Name, d.MinQty))
	}

	perUnit, reason := perUnitDiscount(d, unit.SellPrice, opts)
	if reason != "" {
		return skip(reason)
	}
	price := unit.SellPrice - perUnit
	if price < 0 {
		price = 0
	}
	if price >= unit.SellPrice {
		// POS only applies a tier that is strictly cheaper than the list price, so
		// writing this one would create a permanently inert row.
		return skip("yields no price reduction")
	}
	it.TierPrice = price
	it.Convert = true
	return it
}

// resolveUnit returns the unit the tier will price: the discount's threshold
// unit when set, else the product's base unit (which is what a NULL
// min_qty_unit_id means).
func resolveUnit(
	d model.ProductDiscount,
	unitByID map[string]model.ProductUnit,
	baseByProduct map[string]model.ProductUnit,
) (model.ProductUnit, bool) {
	if d.MinQtyUnitID != nil && *d.MinQtyUnitID != "" {
		u, ok := unitByID[*d.MinQtyUnitID]
		return u, ok
	}
	u, ok := baseByProduct[d.ProductID]
	return u, ok
}

// perUnitDiscount converts a discount rule into an amount off ONE unit, or
// returns a skip reason when it has no per-unit form. Rounding mirrors
// sale.lineDiscountAmount's per-item branch (round half up, clamped to the unit
// price) so a converted PERCENT tier charges what POS charges today.
func perUnitDiscount(d model.ProductDiscount, listPrice int64, opts Options) (int64, string) {
	if d.Value < 0 {
		return 0, "invalid discount value"
	}
	typ := d.DiscountType
	if typ == "" {
		typ = discountFixed
	}
	switch typ {
	case discountPercent:
		if d.Value > 10000 { // > 100.00%
			return 0, "invalid discount percentage"
		}
		amt := (listPrice*d.Value + 5000) / 10000
		return clamp(amt, listPrice), ""
	case discountFixed:
		if d.PerItem {
			return clamp(d.Value, listPrice), ""
		}
		if !opts.IncludeLineFixed {
			return 0, "fixed amount applies to the whole line, not per item (use --include-line-fixed)"
		}
		// Exact at the threshold, progressively more generous above it — the
		// documented trade-off of opting in.
		amt := (d.Value + int64(d.MinQty)/2) / int64(d.MinQty)
		return clamp(amt, listPrice), ""
	default:
		return 0, "unknown discount type " + typ
	}
}

func clamp(v, max int64) int64 {
	if v < 0 {
		return 0
	}
	if v > max {
		return max
	}
	return v
}

func tierKey(unitID string, minQty int32) string {
	return fmt.Sprintf("%s|%d", unitID, minQty)
}

// Apply writes the plan in ONE transaction: each convertible discount becomes a
// tier and is then deleted. All-or-nothing — a partial run would leave the shop
// with a mix of the two rules and no record of which is which. Returns the
// number of discounts converted.
func Apply(db *gorm.DB, plan *Plan) (int, error) {
	items := plan.Convertible()
	if len(items) == 0 {
		return 0, nil
	}
	now := time.Now()
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, it := range items {
			t := &model.ProductPriceTier{
				ProductID:     it.ProductID,
				ProductUnitID: it.UnitID,
				UnitName:      it.UnitName,
				UnitFactor:    it.UnitFactor,
				MinQty:        it.MinQty,
				Price:         it.TierPrice,
			}
			if err := tx.Create(t).Error; err != nil {
				return fmt.Errorf("create tier for discount %s: %w", it.DiscountID, err)
			}
			// Open the rung's price history, exactly as CreateProductPriceTier does
			// — otherwise a converted ladder starts with no price on record and its
			// first UI edit looks like the price appeared from nowhere. changedBy is
			// "" (NULL): this is a system conversion, not a user's edit.
			if err := common.RecordTierPrice(tx, t, "", now); err != nil {
				return fmt.Errorf("record tier price for discount %s: %w", it.DiscountID, err)
			}
			if err := tx.Where("id = ?", it.DiscountID).Delete(&model.ProductDiscount{}).Error; err != nil {
				return fmt.Errorf("delete discount %s: %w", it.DiscountID, err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

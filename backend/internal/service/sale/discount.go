package sale

import (
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// lineDiscountAmount returns the RESOLVED discount amount (minor units) on a line
// of `gross` (= qty × unit price). value is minor units when FIXED, basis points
// (percent*100) when PERCENT. When perItem is true the discount applies to EACH
// item (rounded per item, ×qty) instead of the whole line. Clamped to [0, gross].
// Mirrors the purchasing module's lineNetSubtotal so the two domains stay
// consistent (the 4 modes = FIXED/PERCENT × perItem).
func lineDiscountAmount(gross int64, qty int32, perItem bool, discType string, value int64) (int64, error) {
	if value < 0 {
		return 0, common.TokenError(connect.CodeInvalidArgument, "sale.discount_negative")
	}
	t := discType
	if t == "" {
		t = discountFixed
	}
	if t != discountFixed && t != discountPercent {
		return 0, common.TokenError(connect.CodeInvalidArgument, "sale.discount_type_invalid")
	}
	if t == discountPercent && value > 10000 { // > 100.00%
		return 0, common.TokenError(connect.CodeInvalidArgument, "sale.discount_percent_range")
	}

	var disc int64
	if perItem && qty > 0 {
		perItemGross := gross / int64(qty) // exact: gross = qty × unit price
		var pid int64
		if t == discountFixed {
			pid = value
		} else {
			pid = (perItemGross*value + 5000) / 10000 // round half up, per item
		}
		if pid > perItemGross {
			pid = perItemGross
		}
		disc = pid * int64(qty)
	} else if t == discountFixed {
		disc = value
	} else {
		disc = (gross*value + 5000) / 10000 // round half up
	}
	if disc < 0 {
		disc = 0
	}
	if disc > gross {
		disc = gross
	}
	return disc, nil
}

type resolvedDiscount struct {
	discType string
	value    int64
	perItem  bool
	amount   int64
}

// bestProductDiscount returns the largest qualifying, non-expired product
// discount for the line (or found=false when none applies). The min-qty rule is
// unit-aware and compares in BASE units: a discount qualifies when
// baseQty ≥ min_qty × min_qty_unit_factor (min_qty 0 = no rule; e.g. "min 3 box
// ×12" = 36 base units, satisfied whether sold as 3 box or 36 pcs) and it isn't
// expired (expires_at null or ≥ today, server-local). A stored discount that
// fails validation is skipped.
func bestProductDiscount(tx *gorm.DB, productID string, qty, baseQty int32, unitPrice int64) (resolvedDiscount, bool, error) {
	var rows []model.ProductDiscount
	if err := tx.Where("product_id = ?", productID).Find(&rows).Error; err != nil {
		return resolvedDiscount{}, false, connect.NewError(connect.CodeInternal, err)
	}
	if baseQty <= 0 {
		baseQty = qty // back-compat for rows created before UOM
	}
	today := time.Now().Format(common.DateLayout)
	gross := int64(qty) * unitPrice
	var best resolvedDiscount
	found := false
	for i := range rows {
		d := rows[i]
		if d.ExpiresAt != nil && d.ExpiresAt.Format(common.DateLayout) < today {
			continue // expired
		}
		unitFactor := d.MinQtyUnitFactor
		if unitFactor < 1 {
			unitFactor = 1
		}
		if d.MinQty > 0 && int64(baseQty) < int64(d.MinQty)*unitFactor {
			continue // threshold not met
		}
		amt, err := lineDiscountAmount(gross, qty, d.PerItem, d.DiscountType, d.Value)
		if err != nil {
			continue // skip an invalid stored discount
		}
		if amt > best.amount {
			best = resolvedDiscount{discType: d.DiscountType, value: d.Value, perItem: d.PerItem, amount: amt}
			found = true
		}
	}
	return best, found, nil
}

// recomputeLine sets item.LineDiscount + LineTotal (and the discount info). A
// MANUAL line keeps the cashier's stored type/value; otherwise the best product
// discount is auto-applied (or cleared when none qualifies). Caller persists the
// item afterwards. Does NOT touch sale totals — call recomputeSaleTotals after.
func recomputeLine(tx *gorm.DB, item *model.SaleItem) error {
	gross := int64(item.Qty) * item.UnitPriceSnapshot
	if item.DiscountManual {
		amt, err := lineDiscountAmount(gross, item.Qty, false, item.DiscountType, item.DiscountValue)
		if err != nil {
			return err
		}
		item.DiscountPerItem = false
		item.LineDiscount = amt
		item.LineTotal = gross - amt
		return nil
	}
	d, found, err := bestProductDiscount(tx, item.ProductID, item.Qty, item.BaseQty, item.UnitPriceSnapshot)
	if err != nil {
		return err
	}
	if found {
		item.DiscountType = d.discType
		item.DiscountValue = d.value
		item.DiscountPerItem = d.perItem
		item.LineDiscount = d.amount
	} else {
		item.DiscountType = discountFixed
		item.DiscountValue = 0
		item.DiscountPerItem = false
		item.LineDiscount = 0
	}
	item.LineTotal = gross - item.LineDiscount
	return nil
}

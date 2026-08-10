package common

import (
	"time"

	"gorm.io/gorm"

	"github.com/justmart/backend/internal/model"
)

// Grosir tier price versioning, shared by the ProductPriceTierService handlers
// and the `discount-to-grosir` CLI — the two things in the codebase that create
// tiers. It lives here so a tier written by the CLI starts its history the same
// way one written by the UI does; a second copy of this would silently leave
// converted ladders with no opening price on record.
//
// The history is keyed by the RUNG — (product_unit_id, min_qty) — not by the
// tier row, because tiers are hard-deleted. Exactly one row per rung is open
// (effective_to IS NULL), enforced by product_tier_prices_open_idx.

// CloseTierRung closes the open history row for a rung, if one exists. On its
// own this means "this rung stopped having a price" (a deleted or moved tier).
func CloseTierRung(tx *gorm.DB, unitID string, minQty int32, at time.Time) error {
	return tx.Model(&model.ProductTierPrice{}).
		Where("product_unit_id = ? AND min_qty = ? AND effective_to IS NULL", unitID, minQty).
		Update("effective_to", at).Error
}

// RecordTierPrice closes the rung's open row and opens a new one at the tier's
// current price — the same close-open-row versioning product_unit_prices uses
// for sell prices. changedBy is "" for a system write (the CLI, the migration
// backfill), which stores NULL.
func RecordTierPrice(tx *gorm.DB, t *model.ProductPriceTier, changedBy string, at time.Time) error {
	if err := CloseTierRung(tx, t.ProductUnitID, t.MinQty, at); err != nil {
		return err
	}
	row := model.ProductTierPrice{
		ProductID:     t.ProductID,
		ProductUnitID: t.ProductUnitID,
		UnitName:      t.UnitName,
		MinQty:        t.MinQty,
		Price:         t.Price,
		EffectiveFrom: at,
	}
	if changedBy != "" {
		row.ChangedBy = &changedBy
	}
	return tx.Create(&row).Error
}

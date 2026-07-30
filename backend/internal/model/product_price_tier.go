package model

import "time"

// ProductPriceTier is a "grosir" (wholesale) quantity price tier, defined in the
// Product detail "Grosir" tab and auto-applied at POS. A product unit can have
// several tiers forming a ladder (pcs: ≥12 → 8500, ≥60 → 8000, ≥144 → 7200).
//
// A tier REPLACES the line's unit price for the whole line once qty reaches
// MinQty — a price, not a discount, so gross = qty × price stays exact. It binds
// only to a line of its OWN unit (a pcs tier is not earned by buying 1 box,
// unlike ProductDiscount.MinQty which compares in base units), so MinQty is
// counted in that unit and must be ≥ 2 (DB CHECK): a 0/1 tier would just be the
// unit's sell_price, and sale_items.tier_min_qty = 0 is the "no grosir" flag.
//
// UnitName/UnitFactor are display snapshots; the POS resolver never reads them.
type ProductPriceTier struct {
	ID            string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ProductID     string `gorm:"not null;type:uuid;column:product_id"`
	ProductUnitID string `gorm:"not null;type:uuid;column:product_unit_id"`
	UnitName      string `gorm:"not null;default:'';column:unit_name"`
	UnitFactor    int64  `gorm:"not null;default:1;column:unit_factor"`
	MinQty        int32  `gorm:"not null;column:min_qty"`
	Price         int64  `gorm:"not null;default:0"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (ProductPriceTier) TableName() string { return "product_price_tiers" }

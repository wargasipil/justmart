package model

import "time"

// ProductTierPrice is the grosir (wholesale) tier price history, mirroring
// ProductUnitPrice but keyed by the RUNG — (ProductUnitID, MinQty) — rather than
// by a ProductPriceTier row. Tiers are hard-deleted, so a tier id would dangle;
// the rung is what actually carries a price over time, and keying on it keeps the
// history continuous across a delete-then-recreate and makes a moved threshold
// read as "the >=12 rung closed, the >=24 rung opened".
//
// Exactly one open row (EffectiveTo == nil) per rung, enforced by the partial
// unique index product_tier_prices_open_idx. ChangedBy is nullable so the
// migration backfill can seed a baseline with no user.
type ProductTierPrice struct {
	ID            string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ProductID     string     `gorm:"not null;type:uuid;column:product_id"`
	ProductUnitID string     `gorm:"not null;type:uuid;column:product_unit_id"`
	UnitName      string     `gorm:"not null;default:'';column:unit_name"`
	MinQty        int32      `gorm:"not null;column:min_qty"`
	Price         int64      `gorm:"not null"`
	EffectiveFrom time.Time  `gorm:"not null;column:effective_from"`
	EffectiveTo   *time.Time `gorm:"column:effective_to"` // NULL = current/open
	ChangedBy     *string    `gorm:"type:uuid;column:changed_by"`
	CreatedAt     time.Time
}

func (ProductTierPrice) TableName() string { return "product_tier_prices" }

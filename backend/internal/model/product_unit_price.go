package model

import "time"

// ProductUnitPrice is the per-unit sell-price history, mirroring ProductPrice
// but keyed by a product_unit. Exactly one open row (EffectiveTo == nil) per
// unit. ChangedBy is nullable so the migration backfill can seed a baseline.
type ProductUnitPrice struct {
	ID             string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ProductUnitID string     `gorm:"not null;type:uuid;column:product_unit_id"`
	UnitSellPrice  int64      `gorm:"not null;column:unit_sell_price"`
	EffectiveFrom  time.Time  `gorm:"not null;column:effective_from"`
	EffectiveTo    *time.Time `gorm:"column:effective_to"` // NULL = current/open
	ChangedBy      *string    `gorm:"type:uuid;column:changed_by"`
	CreatedAt      time.Time
}

func (ProductUnitPrice) TableName() string { return "product_unit_prices" }

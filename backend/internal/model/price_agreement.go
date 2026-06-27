package model

import "time"

// PriceAgreement is the negotiated/agreed purchase price for a product from a
// supplier, quoted per a chosen purchasable unit (box/strip/pcs). Reference data
// (distinct from ProductLastRestock, which is actuals). Company-wide (no
// warehouse). unit_name/unit_factor are snapshotted from the product_unit at
// write time. One active row per (supplier, product, unit) — enforced by a
// partial unique index.
type PriceAgreement struct {
	ID            string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	SupplierID    string     `gorm:"not null;type:uuid;column:supplier_id"`
	ProductID     string     `gorm:"not null;type:uuid;column:product_id"`
	ProductUnitID string     `gorm:"not null;type:uuid;column:product_unit_id"`
	UnitName      string     `gorm:"not null;default:'';column:unit_name"`
	UnitFactor    int64      `gorm:"not null;default:1;column:unit_factor"`
	Price         int64      `gorm:"not null;default:0"` // agreed cost for 1 of ProductUnit (minor units)
	ValidFrom     *time.Time `gorm:"type:date;column:valid_from"`
	ValidUntil    *time.Time `gorm:"type:date;column:valid_until"`
	Note          string     `gorm:"not null;default:''"`
	Active        bool       `gorm:"not null;default:true"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (PriceAgreement) TableName() string { return "price_agreements" }

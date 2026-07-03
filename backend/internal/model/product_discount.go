package model

import "time"

// ProductDiscount is a per-product discount, defined in the Product detail
// "Discount" tab and auto-applied at POS. A product can have several. The 4 modes
// are (DiscountType FIXED|PERCENT) × PerItem. Value is minor units when FIXED,
// basis points (percent*100) when PERCENT. MinQty is the "minimal items to buy"
// rule (per line, in the selling unit; 0/1 = always). ExpiresAt is optional.
type ProductDiscount struct {
	ID           string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ProductID    string     `gorm:"not null;type:uuid;column:product_id"`
	DiscountType string     `gorm:"not null;default:'PERCENT';column:discount_type"` // FIXED | PERCENT
	PerItem      bool       `gorm:"not null;default:false;column:per_item"`
	Value        int64      `gorm:"not null;default:0"`
	MinQty       int32      `gorm:"not null;default:0;column:min_qty"`
	// Unit of the MinQty threshold (NULL = base unit). Name/factor snapshotted at
	// write time; the POS gate compares base_qty >= MinQty × MinQtyUnitFactor.
	MinQtyUnitID     *string `gorm:"type:uuid;column:min_qty_unit_id"`
	MinQtyUnitName   string  `gorm:"not null;default:'';column:min_qty_unit_name"`
	MinQtyUnitFactor int64   `gorm:"not null;default:1;column:min_qty_unit_factor"`
	ExpiresAt    *time.Time `gorm:"type:date;column:expires_at"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (ProductDiscount) TableName() string { return "product_discounts" }

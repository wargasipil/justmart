package model

import "time"

// ProductUnit defines a sellable/purchasable unit of measure for a product
// and how it converts to the product's base unit. Stock is stored in base
// units; factor is the number of base units per 1 of this unit (base = 1).
type ProductUnit struct {
	ID          string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ProductID  string `gorm:"not null;type:uuid;column:product_id"`
	Name        string `gorm:"not null"`
	Factor      int64  `gorm:"not null"`
	IsBase      bool   `gorm:"not null;default:false;column:is_base"`
	SellPrice   int64  `gorm:"not null;default:0;column:sell_price"`
	Sellable    bool   `gorm:"not null;default:true"`
	Purchasable bool   `gorm:"not null;default:true"`
	SortOrder   int    `gorm:"not null;default:0;column:sort_order"`
	Active      bool   `gorm:"not null;default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (ProductUnit) TableName() string { return "product_units" }

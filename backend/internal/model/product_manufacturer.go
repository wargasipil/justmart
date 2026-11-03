package model

import "time"

// ProductManufacturer is one approved source: this pabrik may make this product.
//
// The APPROVED-SOURCE LIST, distinct from the FACT on a lot. A row here says a
// maker is a legitimate origin for the item, which is what constrains the
// restock form's picker; what actually arrived is recorded on Batch. Keeping
// both answers in one column is what this table was created to undo.
//
// Product.ManufacturerID survives as the PRIMARY maker and is always one of
// these rows -- a denormalized pointer into the set, the same relationship
// Product.UnitPrice has with ProductPrice.
type ProductManufacturer struct {
	ID             string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ProductID      string `gorm:"not null;type:uuid;column:product_id"`
	ManufacturerID string `gorm:"not null;type:uuid;column:manufacturer_id"`
	CreatedAt      time.Time
}

func (ProductManufacturer) TableName() string { return "product_manufacturers" }

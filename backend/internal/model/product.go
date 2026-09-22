package model

import "time"

type Product struct {
	ID                   string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	SKU                  string `gorm:"uniqueIndex;not null;column:sku"`
	Name                 string `gorm:"not null"`
	Unit                 string `gorm:"not null"`
	UnitPrice            int64  `gorm:"not null;column:unit_price"`
	PrescriptionRequired bool   `gorm:"not null;default:false;column:prescription_required"`
	Active               bool   `gorm:"not null;default:true"`
	// Who MADE this product ("pabrik"). Pointer because the column is nullable
	// and NULL is the normal state — a product has a manufacturer only once
	// someone records one. Stored as *string so "" and "unset" stay distinct:
	// UpdateProduct writes NULL to CLEAR the link, and a non-pointer field
	// would make that indistinguishable from "leave it alone".
	ManufacturerID *string `gorm:"column:manufacturer_id"`
	// Denormalized "has a picture, and how fresh" marker. NULL = none. The bytes
	// live in ProductImage; this is what every product read carries instead.
	ImageUpdatedAt *time.Time `gorm:"column:image_updated_at"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Product) TableName() string { return "products" }

// ProductImage holds the two renditions of a product's picture. Separate from
// `products` on purpose — see migration 00051. ImageData is the bounded
// original; ThumbData is the small square the list/POS surfaces read.
type ProductImage struct {
	ProductID   string `gorm:"primaryKey;column:product_id"`
	ContentType string `gorm:"not null"`
	ImageData   []byte `gorm:"not null"`
	ThumbData   []byte `gorm:"not null"`
	UpdatedAt   time.Time
}

func (ProductImage) TableName() string { return "product_images" }

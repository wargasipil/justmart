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
	// What selling one costs the inventory: STOCKED (own batches) | COMPOSITE
	// (explodes through ProductRecipeItem) | SERVICE (nothing). Stored as the
	// bare string like users.role. Empty is read as STOCKED so pre-kind rows and
	// any caller that never sets it keep the original behaviour.
	Kind string `gorm:"not null;default:STOCKED;column:product_kind"`
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

// ProductRecipeItem is one ingredient line of a COMPOSITE product's recipe.
// QtyBase is the component's BASE units per ONE BASE unit of the parent, so a
// sale of N base units consumes N*QtyBase — see migration 00054.
type ProductRecipeItem struct {
	ID                 string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ProductID          string `gorm:"not null;column:product_id"`
	ComponentProductID string `gorm:"not null;column:component_product_id"`
	QtyBase            int64  `gorm:"not null;column:qty_base"`
	Note               string `gorm:"not null;default:''"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (ProductRecipeItem) TableName() string { return "product_recipe_items" }

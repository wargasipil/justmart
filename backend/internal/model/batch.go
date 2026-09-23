package model

import "time"

type Batch struct {
	ID          string  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ProductID  string  `gorm:"not null;type:uuid;column:product_id"`
	SupplierID  *string `gorm:"type:uuid;column:supplier_id"`
	BatchNumber string  `gorm:"not null;default:'';column:batch_number"`
	ExpiryDate  time.Time `gorm:"not null;type:date;column:expiry_date"`
	CostPrice   int64   `gorm:"not null;default:0;column:cost_price"`
	ReceivedAt  time.Time `gorm:"not null;type:date;column:received_at"`
	// Who MADE this lot, stamped from the purchase-order line at receive. NULL
	// on every pre-00060 batch and on the manual CreateBatch path: the fact was
	// never captured then, and deriving it from the product's current maker
	// would invent provenance for stock that predates the question.
	ManufacturerID *string `gorm:"type:uuid;column:manufacturer_id"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Batch) TableName() string { return "batches" }

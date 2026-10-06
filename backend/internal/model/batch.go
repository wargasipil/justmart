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
	// Where ExpiryDate came from: ENTERED | DEFAULT | NONE (common.ExpirySource*).
	// NONE means ExpiryDate is common.NoExpiryDate, not a real date.
	ExpirySource string `gorm:"not null;default:ENTERED;column:expiry_source"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Batch) TableName() string { return "batches" }

// BatchExpiryChange is one correction or confirmation of a lot's expiry
// (SetBatchExpiry). Insert-only; the lot's history of them.
type BatchExpiryChange struct {
	ID              string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	BatchID         string    `gorm:"not null;type:uuid;column:batch_id"`
	OldExpiryDate   time.Time `gorm:"not null;type:date;column:old_expiry_date"`
	OldExpirySource string    `gorm:"not null;column:old_expiry_source"`
	NewExpiryDate   time.Time `gorm:"not null;type:date;column:new_expiry_date"`
	NewExpirySource string    `gorm:"not null;column:new_expiry_source"`
	Reason          string    `gorm:"not null;column:reason"`
	ChangedBy       string    `gorm:"not null;type:uuid;column:changed_by"`
	ChangedAt       time.Time `gorm:"not null;column:changed_at"`
}

func (BatchExpiryChange) TableName() string { return "batch_expiry_changes" }

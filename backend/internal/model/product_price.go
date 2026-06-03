package model

import "time"

type ProductPrice struct {
	ID            string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ProductID    string     `gorm:"not null;type:uuid;column:product_id"`
	UnitPrice     int64      `gorm:"not null;column:unit_price"`
	EffectiveFrom time.Time  `gorm:"not null;column:effective_from"`
	EffectiveTo   *time.Time `gorm:"column:effective_to"` // NULL = current/open
	ChangedBy     string     `gorm:"not null;type:uuid;column:changed_by"`
	CreatedAt     time.Time
}

func (ProductPrice) TableName() string { return "product_prices" }

package model

import "time"

// Manufacturer ("pabrik") — who MADE the goods. Distinct from Supplier, who
// SOLD them to this shop; see migration 00059 for why the two are separate
// tables. No bank/rekening fields on purpose: a shop pays the distributor.
type Manufacturer struct {
	ID           string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Code         string `gorm:"uniqueIndex;not null"`
	Name         string `gorm:"not null"`
	Address      string `gorm:"not null;default:''"`
	Phone        string `gorm:"not null;default:''"`
	ContactEmail string `gorm:"not null;default:'';column:contact_email"`
	Note         string `gorm:"not null;default:''"`
	Active       bool   `gorm:"not null;default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Manufacturer) TableName() string { return "manufacturers" }

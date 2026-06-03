package model

import "time"

type Customer struct {
	ID        string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name      string `gorm:"not null"`
	Phone     string `gorm:"not null;default:''"`
	Address   string `gorm:"not null;default:''"`
	Notes     string `gorm:"not null;default:''"`
	Active    bool   `gorm:"not null;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Customer) TableName() string { return "customers" }

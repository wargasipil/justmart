package model

import "time"

// DiningTable is one table on the floor of an outlet. Tables hang off a
// WAREHOUSE because that is this app's location concept — an outlet's store is
// its warehouse — so "the tables here" is the same scoping question, answered by
// the same X-Warehouse-Id header, as "the stock here".
//
// A table holds no order state of its own: whether it is occupied is derived
// from the existence of a DRAFT sale carrying its id.
type DiningTable struct {
	ID          string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	WarehouseID string `gorm:"not null;type:uuid;column:warehouse_id"`
	Code        string `gorm:"not null"` // floor label ("T1"); unique per outlet among ACTIVE tables
	Name        string `gorm:"not null;default:''"`
	Area        string `gorm:"not null;default:''"`
	Seats       int32  `gorm:"not null;default:0"`
	Active      bool   `gorm:"not null;default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (DiningTable) TableName() string { return "dining_tables" }

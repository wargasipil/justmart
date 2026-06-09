// Package unit implements unit_iface.v1.UnitService (the base/derivative unit
// catalog). One RPC per file; this file holds the struct + constructor.
package unit

import "gorm.io/gorm"

type UnitService struct {
	db *gorm.DB
}

func NewUnitService(db *gorm.DB) *UnitService { return &UnitService{db: db} }

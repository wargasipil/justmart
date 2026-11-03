// Package manufacturer implements inventory_iface.v1.ManufacturerService. One
// RPC per file; this file holds the struct + constructor. Shared helpers come
// from service/common.
//
// A manufacturer ("pabrik") is who MADE the goods, as distinct from a supplier
// ("pemasok") who sold them to this shop — see migration 00059.
package manufacturer

import "gorm.io/gorm"

type ManufacturerService struct {
	db *gorm.DB
}

func NewManufacturerService(db *gorm.DB) *ManufacturerService {
	return &ManufacturerService{db: db}
}

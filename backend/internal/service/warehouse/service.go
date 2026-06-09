// Package warehouse implements warehouse_iface.v1.WarehouseService. One RPC per
// file; this file holds the struct + constructor. The active-warehouse resolver
// lives in service/common (ResolveWarehouse), shared by every stock surface.
package warehouse

import "gorm.io/gorm"

type WarehouseService struct {
	db *gorm.DB
}

func NewWarehouseService(db *gorm.DB) *WarehouseService { return &WarehouseService{db: db} }

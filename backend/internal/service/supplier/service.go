// Package supplier implements inventory_iface.v1.SupplierService. One RPC per
// file; this file holds the struct + constructor. Shared helpers come from
// service/common.
package supplier

import "gorm.io/gorm"

type SupplierService struct {
	db *gorm.DB
}

func NewSupplierService(db *gorm.DB) *SupplierService { return &SupplierService{db: db} }

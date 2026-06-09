// Package stock implements inventory_iface.v1.StockMovementService (the
// insert-only stock ledger). One RPC per file; this file holds the struct +
// constructor.
package stock

import "gorm.io/gorm"

type StockService struct {
	db *gorm.DB
}

func NewStockService(db *gorm.DB) *StockService { return &StockService{db: db} }

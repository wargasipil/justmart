// Package product implements inventory_iface.v1.ProductService. One RPC per
// file; this file holds the struct + constructor. Non-RPC helpers live in
// helpers.go (converters/load), enrich.go (stock/stocktake enrichment) and
// units.go (unit + per-unit price versioning).
package product

import "gorm.io/gorm"

type ProductService struct {
	db *gorm.DB
}

func NewProductService(db *gorm.DB) *ProductService { return &ProductService{db: db} }

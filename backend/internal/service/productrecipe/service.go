// Package productrecipe implements inventory_iface.v1.ProductRecipeService — CRUD
// for the bill of materials of a COMPOSITE product (the Product detail "Resep"
// card). The recipe is what a completed sale explodes through: selling one
// portion consumes each component's qty_base from the ledger, so food cost and
// COGS fall out of the existing stock_movements chain with no special-casing in
// analytics. Mirrors the productpricetier domain's per-RPC-file + co-located-test
// layout.
package productrecipe

import "gorm.io/gorm"

type ProductRecipeService struct {
	db *gorm.DB
}

func NewProductRecipeService(db *gorm.DB) *ProductRecipeService {
	return &ProductRecipeService{db: db}
}

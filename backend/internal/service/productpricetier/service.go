// Package productpricetier implements inventory_iface.v1.ProductPriceTierService
// — CRUD for "grosir" (wholesale) quantity price tiers (the Product detail
// "Grosir" tab). Tiers are auto-applied at POS by the sale service, which
// replaces the line's unit price when the qty reaches a tier's threshold.
// Mirrors the productdiscount domain's per-RPC-file + co-located-test layout.
package productpricetier

import "gorm.io/gorm"

type ProductPriceTierService struct {
	db *gorm.DB
}

func NewProductPriceTierService(db *gorm.DB) *ProductPriceTierService {
	return &ProductPriceTierService{db: db}
}

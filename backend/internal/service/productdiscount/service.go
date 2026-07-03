// Package productdiscount implements inventory_iface.v1.ProductDiscountService —
// CRUD for per-product discounts (the Product detail "Discount" tab). Discounts
// are auto-applied at POS by the sale service. Mirrors the priceagreement
// domain's per-RPC-file + co-located-test layout.
package productdiscount

import "gorm.io/gorm"

type ProductDiscountService struct {
	db *gorm.DB
}

func NewProductDiscountService(db *gorm.DB) *ProductDiscountService {
	return &ProductDiscountService{db: db}
}

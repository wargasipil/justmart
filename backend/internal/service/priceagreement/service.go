// Package priceagreement implements inventory_iface.v1.PriceAgreementService —
// CRUD for negotiated supplier purchase prices (per product, per purchasable
// unit). Reference data only; it does not feed the PO flow. Mirrors the supplier
// domain's per-RPC-file + co-located-test layout.
package priceagreement

import "gorm.io/gorm"

type PriceAgreementService struct {
	db *gorm.DB
}

func NewPriceAgreementService(db *gorm.DB) *PriceAgreementService {
	return &PriceAgreementService{db: db}
}

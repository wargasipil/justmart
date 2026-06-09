// Package sale implements pos_iface.v1.SaleService (POS / order history). One
// RPC per file; this file holds the struct, constructor, and local aliases for
// the sale status / payment / movement-type constants (source of truth in
// service/common). The abandoned-DRAFT sweeper lives in sweeper.go.
package sale

import (
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/service/common"
)

const (
	saleStatusDraft     = common.SaleStatusDraft
	saleStatusCompleted = common.SaleStatusCompleted
	saleStatusVoided    = common.SaleStatusVoided

	paymentCash    = common.PaymentCash
	paymentNonCash = common.PaymentNonCash

	movementTypeSale = common.MovementTypeSale
)

type SaleService struct {
	db      *gorm.DB
	printer config.Printer
}

func NewSaleService(db *gorm.DB, printerCfg config.Printer) *SaleService {
	return &SaleService{db: db, printer: printerCfg}
}

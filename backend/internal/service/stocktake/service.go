// Package stocktake implements stocktake_iface.v1.StocktakeService. One RPC per
// file; this file holds the struct, constructor, and stocktake-domain constants.
package stocktake

import "gorm.io/gorm"

const (
	stocktakeStatusDraft     = "DRAFT"
	stocktakeStatusCompleted = "COMPLETED"
	stocktakeStatusVoided    = "VOIDED"

	dispositionAdjustment = "ADJUSTMENT"
	dispositionWriteOff   = "WRITE_OFF"
)

var validWriteOffKinds = map[string]bool{
	"EXPIRED": true,
	"DAMAGED": true,
	"LOST":    true,
	"THEFT":   true,
	"OTHER":   true,
}

type StocktakeService struct {
	db *gorm.DB
}

func NewStocktakeService(db *gorm.DB) *StocktakeService { return &StocktakeService{db: db} }

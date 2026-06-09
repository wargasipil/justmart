// Package transfer implements warehouse_iface.v1.StockTransferService (moving
// stock between warehouses). One RPC per file; this file holds the struct +
// constructor + the transfer-local movement-type constants.
package transfer

import "gorm.io/gorm"

const (
	movementTransferOut = "TRANSFER_OUT"
	movementTransferIn  = "TRANSFER_IN"
)

type TransferService struct {
	db *gorm.DB
}

func NewTransferService(db *gorm.DB) *TransferService { return &TransferService{db: db} }

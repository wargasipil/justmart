// Package purchasing implements purchasing_iface.v1's three services —
// PurchaseOrderService, PurchaseReceiptService, PurchasePaymentService — in a
// single package because they share PO helpers (lockByID, recomputePOStatus,
// maybeCloseIfPaid, resolvePurchaseUnit) and PayPurchase reaches into the
// PurchaseOrders helper set. One RPC per file; shared helpers in helpers.go /
// convert.go. PO status constants alias service/common.
package purchasing

import (
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/service/common"
)

const (
	poStatusDraft             = common.POStatusDraft
	poStatusSent              = common.POStatusSent
	poStatusPartiallyReceived = common.POStatusPartiallyReceived
	poStatusReceived          = common.POStatusReceived
	poStatusClosed            = common.POStatusClosed
	poStatusVoided            = common.POStatusVoided

	defaultPPNRate = 11 // Indonesia's standard PPN rate as of 2026
)

type PurchaseOrders struct {
	db *gorm.DB
}

func NewPurchaseOrderService(db *gorm.DB) *PurchaseOrders { return &PurchaseOrders{db: db} }

type PurchaseReceipts struct {
	db *gorm.DB
}

func NewPurchaseReceiptService(db *gorm.DB) *PurchaseReceipts { return &PurchaseReceipts{db: db} }

type PurchasePayments struct {
	db *gorm.DB
}

func NewPurchasePaymentService(db *gorm.DB) *PurchasePayments { return &PurchasePayments{db: db} }

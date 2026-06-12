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

// ConnectorPusher is the print-connector registry seam used by PrintReceipt
// when connector mode is on. *connector.ConnectorService satisfies it; tests
// pass a fake. Kept as an interface so the sale package doesn't import the
// connector package.
type ConnectorPusher interface {
	Push(deviceID, printerName string, payload []byte) (jobID string, err error)
}

type SaleService struct {
	db           *gorm.DB
	printer      config.Printer
	connectorCfg config.Connector
	connector    ConnectorPusher
}

func NewSaleService(db *gorm.DB, printerCfg config.Printer) *SaleService {
	return &SaleService{db: db, printer: printerCfg}
}

// SetConnector wires the print-connector path. PrintReceipt routes to the
// connector when connectorCfg.Mode == "connector"; otherwise the default
// raw-TCP path is used. Called once from serve.go after the registry is built
// (and from tests with a fake pusher).
func (s *SaleService) SetConnector(connectorCfg config.Connector, pusher ConnectorPusher) {
	s.connectorCfg = connectorCfg
	s.connector = pusher
}

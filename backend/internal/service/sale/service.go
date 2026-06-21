// Package sale implements pos_iface.v1.SaleService (POS / order history). One
// RPC per file; this file holds the struct, constructor, and local aliases for
// the sale status / payment / movement-type constants (source of truth in
// service/common). The abandoned-DRAFT sweeper lives in sweeper.go.
package sale

import (
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/spooler"
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

// SpoolFunc prints rendered receipt bytes to a locally-installed printer (the
// usb-mode seam). Defaults to spooler.Print (Windows spooler; a no-op error off
// Windows); tests inject a fake via SetSpooler.
type SpoolFunc func(printerName string, payload []byte, jobID string) error

type SaleService struct {
	db           *gorm.DB
	printer      config.Printer
	connectorCfg config.Connector
	connector    ConnectorPusher
	spool        SpoolFunc
}

func NewSaleService(db *gorm.DB, printerCfg config.Printer) *SaleService {
	return &SaleService{db: db, printer: printerCfg, spool: spooler.Print}
}

// SetConnector wires the print-connector path. PrintReceipt routes to the
// connector when connectorCfg.Mode == "connector"; otherwise the default
// raw-TCP path is used. Called once from serve.go after the registry is built
// (and from tests with a fake pusher).
func (s *SaleService) SetConnector(connectorCfg config.Connector, pusher ConnectorPusher) {
	s.connectorCfg = connectorCfg
	s.connector = pusher
}

// SetSpooler overrides the usb-mode local print function (tests inject a fake;
// production uses the spooler.Print default set in NewSaleService).
func (s *SaleService) SetSpooler(fn SpoolFunc) { s.spool = fn }

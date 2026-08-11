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
	saleStatusRefunded  = common.SaleStatusRefunded

	paymentCash    = common.PaymentCash
	paymentNonCash = common.PaymentNonCash

	movementTypeSale   = common.MovementTypeSale
	movementTypeReturn = common.MovementTypeReturn
)

// ConnectorPusher / SpoolFunc are the print seams, aliased to the shared
// definitions in service/common so the sale package doesn't import the
// connector package and callers/tests keep the same names.
type (
	ConnectorPusher = common.ConnectorPusher
	SpoolFunc       = common.SpoolFunc
)

type SaleService struct {
	db      *gorm.DB
	printer config.Printer
	// print routes rendered bytes to connector / usb spooler / raw TCP. Shared
	// with ProductService.PrintProductLabel so target precedence stays identical.
	print *common.PrintDispatcher
}

func NewSaleService(db *gorm.DB, printerCfg config.Printer) *SaleService {
	return &SaleService{db: db, printer: printerCfg, print: common.NewPrintDispatcher(printerCfg)}
}

// SetConnector wires the print-connector path. PrintReceipt routes to the
// connector when connectorCfg.Mode == "connector"; otherwise the default
// raw-TCP path is used. Called once from serve.go after the registry is built
// (and from tests with a fake pusher).
func (s *SaleService) SetConnector(connectorCfg config.Connector, pusher ConnectorPusher) {
	s.print.SetConnector(connectorCfg, pusher)
}

// SetSpooler overrides the usb-mode local print function (tests inject a fake;
// production uses the spooler.Print default set in NewSaleService).
func (s *SaleService) SetSpooler(fn SpoolFunc) { s.print.SetSpooler(fn) }

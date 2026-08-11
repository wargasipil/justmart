// Package product implements inventory_iface.v1.ProductService. One RPC per
// file; this file holds the struct + constructor. Non-RPC helpers live in
// helpers.go (converters/load), enrich.go (stock/stocktake enrichment) and
// units.go (unit + per-unit price versioning).
package product

import (
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/service/common"
)

// ConnectorPusher / SpoolFunc are the print seams, aliased to the shared
// definitions in service/common (see PrintProductLabel).
type (
	ConnectorPusher = common.ConnectorPusher
	SpoolFunc       = common.SpoolFunc
)

type ProductService struct {
	db *gorm.DB
	// print backs PrintProductLabel. Nil until SetPrinter runs, so every other
	// RPC — and every test that doesn't print — needs no printer config.
	print *common.PrintDispatcher
}

func NewProductService(db *gorm.DB) *ProductService { return &ProductService{db: db} }

// SetPrinter enables PrintProductLabel. Called once at boot with the shop's
// printer config + the connector registry (and from tests with a fake pusher).
func (s *ProductService) SetPrinter(
	printerCfg config.Printer,
	connectorCfg config.Connector,
	pusher ConnectorPusher,
) {
	s.print = common.NewPrintDispatcher(printerCfg)
	s.print.SetConnector(connectorCfg, pusher)
}

// SetSpooler overrides the usb-mode local print function (tests inject a fake).
// SetPrinter must have run first.
func (s *ProductService) SetSpooler(fn SpoolFunc) { s.print.SetSpooler(fn) }

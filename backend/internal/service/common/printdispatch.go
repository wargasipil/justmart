package common

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/printer"
	"github.com/justmart/backend/internal/spooler"
)

// ConnectorPusher is the print-connector registry seam. *connector.ConnectorService
// satisfies it; tests pass a fake. Kept as an interface so service packages
// don't import the connector package.
type ConnectorPusher interface {
	Push(deviceID, printerName string, payload []byte) (jobID string, err error)
}

// SpoolFunc prints rendered bytes to a locally-installed printer (the usb-mode
// seam). Production uses spooler.Print (Windows spooler; a no-op error off
// Windows); tests inject a fake.
type SpoolFunc func(printerName string, payload []byte, jobID string) error

// PrintDispatcher routes already-rendered ESC/POS bytes to whichever printer
// path the shop is configured for — the print connector, the local OS spooler,
// or a raw-TCP network printer.
//
// It exists so receipts and product labels cannot drift apart: target
// precedence (request → saved Settings default → config → sole device) is
// subtle enough that a second copy of it would be a bug waiting to happen.
type PrintDispatcher struct {
	printerCfg   config.Printer
	connectorCfg config.Connector
	connector    ConnectorPusher
	spool        SpoolFunc
}

func NewPrintDispatcher(printerCfg config.Printer) *PrintDispatcher {
	return &PrintDispatcher{printerCfg: printerCfg, spool: spooler.Print}
}

// SetConnector wires the print-connector path. Dispatch routes to the connector
// when connectorCfg.Mode == "connector"; otherwise the usb / raw-TCP paths are
// used. Called once at boot after the registry is built (and from tests with a
// fake pusher).
func (d *PrintDispatcher) SetConnector(connectorCfg config.Connector, pusher ConnectorPusher) {
	d.connectorCfg = connectorCfg
	d.connector = pusher
}

// SetSpooler overrides the usb-mode local print function (tests inject a fake).
func (d *PrintDispatcher) SetSpooler(fn SpoolFunc) { d.spool = fn }

// PrinterWidth is the configured paper width in characters, for renderers that
// need it before a settings lookup.
func (d *PrintDispatcher) PrinterWidth() int { return d.printerCfg.Width }

// EnsureConfigured reports whether printing is usable at all. tcp is the only
// mode that needs printer.enabled; connector/usb are gated by the mode itself.
func (d *PrintDispatcher) EnsureConfigured() error {
	mode := d.connectorCfg.Mode
	if mode != "connector" && mode != "usb" && !d.printerCfg.Enabled {
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("printing is not configured (set printer.enabled, or connector.mode to connector/usb, in config.yaml)"))
	}
	return nil
}

// Dispatch sends payload to the configured printer. deviceID/printerName are the
// caller's explicit target; empty values fall back to the saved Settings
// default, then to config, then to the sole connected connector. jobID labels
// the spooler job. Errors come back as connect errors ready to return.
func (d *PrintDispatcher) Dispatch(
	ctx context.Context,
	db *gorm.DB,
	deviceID, printerName string,
	payload []byte,
	jobID string,
) error {
	switch d.connectorCfg.Mode {
	case "connector":
		if d.connector == nil {
			return connect.NewError(connect.CodeFailedPrecondition,
				errors.New("connector mode is on but no print connector registry is wired"))
		}
		// No explicit target on the request → fall back to the saved default.
		if deviceID == "" {
			saved, savedPrinter, err := GetPrintTarget(ctx, db)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			deviceID = saved
			if printerName == "" {
				printerName = savedPrinter
			}
		}
		// deviceID may still be "" → Push targets the sole connected connector.
		if _, err := d.connector.Push(deviceID, printerName, payload); err != nil {
			return err // already a connect error (Unavailable)
		}
	case "usb":
		// Print straight to a locally-installed printer via the OS spooler
		// (Windows only; the !windows spooler stub errors out). Printer
		// precedence: request → saved Settings default → config.printer_name →
		// "" (host default).
		if printerName == "" {
			_, saved, err := GetPrintTarget(ctx, db)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			printerName = saved
		}
		if printerName == "" {
			printerName = d.connectorCfg.PrinterName
		}
		if err := d.spool(printerName, payload, jobID); err != nil {
			return connect.NewError(connect.CodeUnavailable, err)
		}
	default: // "tcp"
		if err := printer.DispatchTCP(d.printerCfg.Address, payload, d.printerCfg.Timeout); err != nil {
			return connect.NewError(connect.CodeUnavailable, err)
		}
	}
	return nil
}

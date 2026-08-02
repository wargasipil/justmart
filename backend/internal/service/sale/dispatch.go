package sale

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/justmart/backend/internal/printer"
	"github.com/justmart/backend/internal/service/common"
)

// printTarget is where a rendered payload should come out. Both fields may be
// empty: an empty device means "the sole connected connector", an empty printer
// means "that device's default".
type printTarget struct {
	DeviceID    string
	PrinterName string
}

// savedTarget picks the fallback target for a document type. Customer receipts
// and kitchen tickets deliberately resolve to DIFFERENT saved defaults — the
// whole point of a kitchen ticket is that it comes out at the pass, not at the
// till — but the resolution ORDER is the same for both, so it lives here once.
type savedTargetFunc func(ctx context.Context) (deviceID, printerName string, err error)

// dispatch sends a rendered ESC/POS payload out through whichever print mode the
// shop is configured for.
//
// Extracted so the receipt and the kitchen ticket cannot drift: they are
// different DOCUMENTS with different content and different default targets, but
// getting the bytes to a printer is one problem, and it was 45 lines of mode
// branching that would otherwise have been copied.
//
// `ref` is an opaque label used only for spooler job naming.
func (s *SaleService) dispatch(
	ctx context.Context,
	req printTarget,
	saved savedTargetFunc,
	payload []byte,
	ref string,
) error {
	switch s.connectorCfg.Mode {
	case "connector":
		if s.connector == nil {
			return connect.NewError(connect.CodeFailedPrecondition,
				errors.New("connector mode is on but no print connector registry is wired"))
		}
		deviceID, printerName := req.DeviceID, req.PrinterName
		// No explicit target on the request → fall back to the saved default.
		if deviceID == "" {
			d, p, err := saved(ctx)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			deviceID = d
			if printerName == "" {
				printerName = p
			}
		}
		// deviceID may still be "" → Push targets the sole connected connector.
		if _, err := s.connector.Push(deviceID, printerName, payload); err != nil {
			return err // already a connect error (Unavailable)
		}
	case "usb":
		// Print straight to a locally-installed printer via the OS spooler
		// (Windows only; the !windows spooler stub errors out). Printer precedence:
		// request → saved Settings default → config.printer_name → "" (host default).
		printerName := req.PrinterName
		if printerName == "" {
			_, p, err := saved(ctx)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			printerName = p
		}
		if printerName == "" {
			printerName = s.connectorCfg.PrinterName
		}
		if err := s.spool(printerName, payload, ref); err != nil {
			return connect.NewError(connect.CodeUnavailable, err)
		}
	default: // "tcp"
		if err := printer.DispatchTCP(s.printer.Address, payload, s.printer.Timeout); err != nil {
			return connect.NewError(connect.CodeUnavailable, err)
		}
	}
	return nil
}

// assertPrintingConfigured is the shared precondition: tcp is the only mode that
// needs printer.enabled, since connector/usb are gated by the mode itself.
func (s *SaleService) assertPrintingConfigured() error {
	mode := s.connectorCfg.Mode
	if mode != "connector" && mode != "usb" && !s.printer.Enabled {
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("printing is not configured (set printer.enabled, or connector.mode to connector/usb, in config.yaml)"))
	}
	return nil
}

// receiptTarget is the saved default for CUSTOMER receipts.
func (s *SaleService) receiptTarget(ctx context.Context) (string, string, error) {
	return common.GetPrintTarget(ctx, s.db)
}

// kitchenTarget is the saved default for KITCHEN tickets, falling back to the
// receipt printer when no kitchen printer has been configured.
//
// The fallback matters: a warung with one printer should get working kitchen
// tickets the moment it switches to restaurant mode, without first hunting
// through Settings. A shop that does have a pass printer sets it once and the
// two documents part ways.
func (s *SaleService) kitchenTarget(ctx context.Context) (string, string, error) {
	d, p, err := common.GetKitchenPrintTarget(ctx, s.db)
	if err != nil {
		return "", "", err
	}
	if d == "" && p == "" {
		return common.GetPrintTarget(ctx, s.db)
	}
	return d, p, nil
}

// printSettings builds the renderer settings from app_settings (seeded at boot
// from config.yaml, editable in Settings ▸ Printing), falling back to the config
// only if a lookup errors. Drawer stays hardware config.
func (s *SaleService) printSettings(ctx context.Context) printer.Settings {
	header, footer := s.printer.Header, s.printer.Footer
	if h, f, err := common.GetReceiptText(ctx, s.db); err == nil {
		header = common.ReceiptLines(h)
		footer = common.ReceiptLines(f)
	}
	width := s.printer.Width
	if w, err := common.GetReceiptWidth(ctx, s.db); err == nil {
		width = int(w)
	}
	return printer.Settings{
		Width:      width,
		Header:     header,
		Footer:     footer,
		OpenDrawer: s.printer.OpenDrawer,
	}
}

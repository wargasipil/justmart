package sale_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/service/common"
	salesvc "github.com/justmart/backend/internal/service/sale"
	"github.com/justmart/backend/internal/service/servicetest"
)

// spoolCapture records what the fake spooler was called with.
type spoolCapture struct {
	called  bool
	printer string
	jobID   string
	payload []byte
}

// usbSetup wires a completed sale + a fake spooler and returns the service, a
// caller ctx, the sale id, the captured spool args, and the db (for tests that
// need to seed a saved print target). Shared setup for the usb-routing tests.
func usbSetup(t *testing.T, mode config.Connector, spoolErr error) (*salesvc.SaleService, context.Context, string, *spoolCapture, *gorm.DB) {
	t.Helper()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := salesvc.NewSaleService(gormDB, cfg.Printer)
	svc.SetConnector(mode, nil) // usb mode doesn't use the connector registry
	got := &spoolCapture{}
	svc.SetSpooler(func(printerName string, payload []byte, jobID string) error {
		got.called, got.printer, got.jobID, got.payload = true, printerName, jobID, payload
		return spoolErr
	})
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	productID := seedProduct(t, gormDB, "usb-sku", "Paracetamol", 2000)
	seedStock(t, gormDB, productID, ownerID, 10)
	saleID := startDraft(t, svc, ctx)
	completeOne(t, svc, ctx, productID, saleID, 1, 5000)
	return svc, ctx, saleID, got, gormDB
}

// TestPrintReceipt_UsbMode_SpoolsToConfiguredPrinter: usb mode renders the
// receipt and spools it to the printer named in config; BytesSent echoes the
// rendered payload length.
func TestPrintReceipt_UsbMode_SpoolsToConfiguredPrinter(t *testing.T) {
	t.Parallel()
	svc, ctx, saleID, got, _ := usbSetup(t, config.Connector{Mode: "usb", PrinterName: "EPSON-TM"}, nil)

	resp, err := svc.PrintReceipt(ctx, connect.NewRequest(&posifacev1.PrintReceiptRequest{SaleId: saleID}))
	require.NoError(t, err)
	require.True(t, got.called)
	require.Equal(t, "EPSON-TM", got.printer)
	require.NotEmpty(t, got.payload) // real rendered receipt bytes
	require.Equal(t, int32(len(got.payload)), resp.Msg.BytesSent)
}

// TestPrintReceipt_UsbMode_RequestOverridesPrinter: a PrinterName on the request
// overrides the configured default.
func TestPrintReceipt_UsbMode_RequestOverridesPrinter(t *testing.T) {
	t.Parallel()
	svc, ctx, saleID, got, _ := usbSetup(t, config.Connector{Mode: "usb", PrinterName: "EPSON-TM"}, nil)

	_, err := svc.PrintReceipt(ctx, connect.NewRequest(&posifacev1.PrintReceiptRequest{
		SaleId: saleID, PrinterName: "OTHER-58",
	}))
	require.NoError(t, err)
	require.Equal(t, "OTHER-58", got.printer)
}

// TestPrintReceipt_UsbMode_BlankPrinterPassesThrough: no configured/request
// printer → "" reaches the spooler (which resolves the host default printer).
func TestPrintReceipt_UsbMode_BlankPrinterPassesThrough(t *testing.T) {
	t.Parallel()
	svc, ctx, saleID, got, _ := usbSetup(t, config.Connector{Mode: "usb"}, nil)

	_, err := svc.PrintReceipt(ctx, connect.NewRequest(&posifacev1.PrintReceiptRequest{SaleId: saleID}))
	require.NoError(t, err)
	require.True(t, got.called)
	require.Equal(t, "", got.printer)
}

// TestPrintReceipt_UsbMode_SavedTargetUsedWhenRequestBlank: with no request
// printer and no config printer_name, the saved Settings print target (its
// printer slot) is used. Precedence: request → saved → config → host default.
func TestPrintReceipt_UsbMode_SavedTargetUsedWhenRequestBlank(t *testing.T) {
	t.Parallel()
	svc, ctx, saleID, got, db := usbSetup(t, config.Connector{Mode: "usb"}, nil)
	require.NoError(t, common.SetPrintTarget(ctx, db, "", "SAVED-LOCAL"))

	_, err := svc.PrintReceipt(ctx, connect.NewRequest(&posifacev1.PrintReceiptRequest{SaleId: saleID}))
	require.NoError(t, err)
	require.Equal(t, "SAVED-LOCAL", got.printer)
}

// TestPrintReceipt_UsbMode_SpoolErrorUnavailable: a spooler failure (e.g. the
// !windows stub, or no such printer) surfaces as Unavailable.
func TestPrintReceipt_UsbMode_SpoolErrorUnavailable(t *testing.T) {
	t.Parallel()
	svc, ctx, saleID, _, _ := usbSetup(t, config.Connector{Mode: "usb"}, errors.New("no such printer"))

	_, err := svc.PrintReceipt(ctx, connect.NewRequest(&posifacev1.PrintReceiptRequest{SaleId: saleID}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
}

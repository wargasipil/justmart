package sale_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/config"
	salesvc "github.com/justmart/backend/internal/service/sale"
	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/service/servicetest"
)

// fakePusher is a ConnectorPusher that records the last Push and can be set to
// fail, standing in for the live connector registry.
type fakePusher struct {
	called      bool
	deviceID    string
	printerName string
	payload     []byte
	err         error
}

func (f *fakePusher) Push(deviceID, printerName string, payload []byte) (string, error) {
	f.called = true
	f.deviceID, f.printerName, f.payload = deviceID, printerName, payload
	if f.err != nil {
		return "", f.err
	}
	return "job-1", nil
}

// TestPrintReceipt_ConnectorMode_ExplicitTarget: an explicit request target is
// passed straight to Push, with the rendered ESC/POS bytes; BytesSent echoes
// the payload length.
func TestPrintReceipt_ConnectorMode_ExplicitTarget(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := salesvc.NewSaleService(gormDB, cfg.Printer)
	fake := &fakePusher{}
	svc.SetConnector(config.Connector{Mode: "connector"}, fake)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	productID := seedProduct(t, gormDB, "c-sku-1", "Paracetamol", 2000)
	seedStock(t, gormDB, productID, ownerID, 10)
	saleID := startDraft(t, svc, ctx)
	completeOne(t, svc, ctx, productID, saleID, 1, 5000)

	resp, err := svc.PrintReceipt(ctx, connect.NewRequest(&posifacev1.PrintReceiptRequest{
		SaleId: saleID, ConnectorDeviceId: "dev-7", PrinterName: "POS-58",
	}))
	require.NoError(t, err)
	require.True(t, fake.called)
	require.Equal(t, "dev-7", fake.deviceID)
	require.Equal(t, "POS-58", fake.printerName)
	require.NotEmpty(t, fake.payload) // real rendered receipt bytes
	require.Equal(t, int32(len(fake.payload)), resp.Msg.BytesSent)
}

// TestPrintReceipt_UsesConfiguredHeaderFooter: the receipt header/footer set in
// Settings (app_settings) are rendered into the printed bytes.
func TestPrintReceipt_UsesConfiguredHeaderFooter(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := salesvc.NewSaleService(gormDB, cfg.Printer)
	fake := &fakePusher{}
	svc.SetConnector(config.Connector{Mode: "connector"}, fake)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	require.NoError(t, common.SetReceiptText(ctx, gormDB, "TOKO JAYA\nBandung", "Terima kasih"))

	productID := seedProduct(t, gormDB, "rcpt-1", "Item", 2000)
	seedStock(t, gormDB, productID, ownerID, 10)
	saleID := startDraft(t, svc, ctx)
	completeOne(t, svc, ctx, productID, saleID, 1, 5000)

	_, err := svc.PrintReceipt(ctx, connect.NewRequest(&posifacev1.PrintReceiptRequest{SaleId: saleID}))
	require.NoError(t, err)
	require.True(t, fake.called)
	payload := string(fake.payload)
	require.Contains(t, payload, "TOKO JAYA")
	require.Contains(t, payload, "Bandung")
	require.Contains(t, payload, "Terima kasih")
}

// TestPrintReceipt_ConnectorMode_DefaultTarget: with no target on the request,
// the saved app_settings default is used.
func TestPrintReceipt_ConnectorMode_DefaultTarget(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := salesvc.NewSaleService(gormDB, cfg.Printer)
	fake := &fakePusher{}
	svc.SetConnector(config.Connector{Mode: "connector"}, fake)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	require.NoError(t, common.SetPrintTarget(ctx, gormDB, "saved-dev", "saved-printer"))

	productID := seedProduct(t, gormDB, "c-sku-2", "Amoxicillin", 3000)
	seedStock(t, gormDB, productID, ownerID, 10)
	saleID := startDraft(t, svc, ctx)
	completeOne(t, svc, ctx, productID, saleID, 1, 5000)

	_, err := svc.PrintReceipt(ctx, connect.NewRequest(&posifacev1.PrintReceiptRequest{SaleId: saleID}))
	require.NoError(t, err)
	require.Equal(t, "saved-dev", fake.deviceID)
	require.Equal(t, "saved-printer", fake.printerName)
}

// TestPrintReceipt_ConnectorMode_NotConnected: a Push failure (no connector)
// propagates as Unavailable.
func TestPrintReceipt_ConnectorMode_NotConnected(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := salesvc.NewSaleService(gormDB, cfg.Printer)
	fake := &fakePusher{err: connect.NewError(connect.CodeUnavailable, errors.New("no connector"))}
	svc.SetConnector(config.Connector{Mode: "connector"}, fake)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	productID := seedProduct(t, gormDB, "c-sku-3", "Ibuprofen", 4000)
	seedStock(t, gormDB, productID, ownerID, 10)
	saleID := startDraft(t, svc, ctx)
	completeOne(t, svc, ctx, productID, saleID, 1, 5000)

	_, err := svc.PrintReceipt(ctx, connect.NewRequest(&posifacev1.PrintReceiptRequest{SaleId: saleID}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
}

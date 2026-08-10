package sale_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/config"
	salesvc "github.com/justmart/backend/internal/service/sale"
	"github.com/justmart/backend/internal/service/servicetest"
)

// Pins test-specification.md › Pos: "an order created from a grosir cart keeps
// the grosir price through completion, RECEIPT and order history". Completion
// and order history are covered by TestCompleteSale_GrosirOrderReadBack; this is
// the receipt leg, which is the one that leaves the database — the printed bytes
// are what the customer is handed, so charging 8.500 and printing 10.000 would
// be a dispute at the counter rather than a reporting discrepancy.
//
// It asserts the LIST price is absent as well as the grosir price present:
// asserting only the latter would still pass if the receipt printed both.
func TestPrintReceipt_GrosirPriceOnPrintedReceipt(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := salesvc.NewSaleService(gormDB, cfg.Printer)
	fake := &fakePusher{}
	svc.SetConnector(config.Connector{Mode: "connector"}, fake)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// ladder: list 10.000, rungs at 12 → 8.500, 60 → 8.000, 144 → 7.200.
	productID, _ := ladder(t, gormDB, "grosir-receipt")
	seedStock(t, gormDB, productID, ownerID, 500)

	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 12,
	}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 102000,
	}))
	require.NoError(t, err)

	_, err = svc.PrintReceipt(ctx, connect.NewRequest(&posifacev1.PrintReceiptRequest{SaleId: saleID}))
	require.NoError(t, err)
	require.True(t, fake.called)

	payload := string(fake.payload)
	require.Contains(t, payload, "12 tab x Kopi sachet", "the line is printed at the qty that earned the tier")
	require.Contains(t, payload, "Rp 102.000", "line total + sale total are the grosir price (12 × 8.500)")
	require.NotContains(t, payload, "Rp 120.000", "the list price (12 × 10.000) must never reach the receipt")
}

// The counterpart: below the threshold nothing is discounted, so the receipt
// carries the list price. Without this, a bug that simply never applied a tier
// would leave the test above as the only signal and this case unguarded.
func TestPrintReceipt_BelowGrosirThresholdPrintsListPrice(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := salesvc.NewSaleService(gormDB, cfg.Printer)
	fake := &fakePusher{}
	svc.SetConnector(config.Connector{Mode: "connector"}, fake)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	productID, _ := ladder(t, gormDB, "grosir-receipt-below")
	seedStock(t, gormDB, productID, ownerID, 500)

	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 11, // one short of the 12 rung
	}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 110000,
	}))
	require.NoError(t, err)

	_, err = svc.PrintReceipt(ctx, connect.NewRequest(&posifacev1.PrintReceiptRequest{SaleId: saleID}))
	require.NoError(t, err)

	payload := string(fake.payload)
	require.Contains(t, payload, "11 tab x Kopi sachet")
	require.Contains(t, payload, "Rp 110.000", "11 × the list 10.000")
}

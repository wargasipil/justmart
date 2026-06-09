package sale_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	salesvc "github.com/justmart/backend/internal/service/sale"
	"github.com/justmart/backend/internal/service/servicetest"
)

// TestPrintReceipt_PrinterDisabled is the primary error path: servicetest's
// config sets Printer.Enabled = false, so PrintReceipt short-circuits with
// FailedPrecondition before touching the DB.
func TestPrintReceipt_PrinterDisabled(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "pr-sku-1", "Paracetamol", 2000)
	seedStock(t, db, productID, ownerID, 10)
	saleID := startDraft(t, svc, ctx)
	completeOne(t, svc, ctx, productID, saleID, 1, 5000)

	_, err := svc.PrintReceipt(ctx, connect.NewRequest(&posifacev1.PrintReceiptRequest{
		SaleId: saleID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// TestPrintReceipt_NotCompleted exercises the post-enable guard: a DRAFT sale
// cannot be printed even with the printer enabled.
func TestPrintReceipt_NotCompleted(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	// Enable the printer so we reach the status check (Address stays empty; we
	// never get to dispatch because the DRAFT fails the COMPLETED guard first).
	cfg.Printer.Enabled = true
	svc := salesvc.NewSaleService(gormDB, cfg.Printer)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	saleID := startDraft(t, svc, ctx)

	_, err := svc.PrintReceipt(ctx, connect.NewRequest(&posifacev1.PrintReceiptRequest{
		SaleId: saleID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

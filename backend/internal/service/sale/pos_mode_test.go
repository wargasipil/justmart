package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
)

// These tests pin test-specification.md › Pos: "POS works in pharmacy mode AND
// retail mode" (and, via the engine-agnostic harness, "on postgres and sqlite").
// The discriminator is the business-mode-gated Rx enforcement in assertRxCovers:
// it must enforce prescriptions in pharmacy mode and be a no-op in retail mode.

// RETAIL (default/unspecified mode): a product that happens to carry the
// prescription_required flag is sold normally — POS must NOT block it, since the
// Rx concept doesn't exist in retail. This is the retail-compatibility guarantee.
func TestPos_RetailMode_RxFlaggedProductSellsWithoutPrescription(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	// No setPharmacyMode → default UNSPECIFIED, which behaves as retail.
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	seedStock(t, db, prodID, ownerID, 50)
	saleID := startDraft(t, svc, ctx)

	// Add succeeds with no prescription attached (gate is off in retail).
	addRes, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 3,
	}))
	require.NoError(t, err)
	require.Len(t, addRes.Msg.Sale.Items, 1)

	// And completes normally.
	compRes, err := svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        saleID,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    100000,
	}))
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_COMPLETED, compRes.Msg.Sale.Status)
}

// RETAIL (explicitly set): same guarantee, proven add-AND-complete.
func TestPos_ExplicitRetailMode_RxFlagIgnored(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setRetailMode(t, db)
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	seedStock(t, db, prodID, ownerID, 50)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 1,
	}))
	require.NoError(t, err)
	compRes, err := svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        saleID,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    100000,
	}))
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_COMPLETED, compRes.Msg.Sale.Status)
}

// Mode flip mid-cart: a DRAFT built in retail (Rx-flagged product added with no
// prescription, gate off) must NOT finalize after a switch to pharmacy mode —
// CompleteSale re-asserts Rx coverage and rejects the uncovered product.
func TestPos_ModeFlipMidCart_CompleteBlocksUncoveredRx(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	// Built in retail (default): the Rx-flagged product adds with no prescription.
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	seedStock(t, db, prodID, ownerID, 50)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 1,
	}))
	require.NoError(t, err)

	// Shop switches to pharmacy mode before checkout.
	setPharmacyMode(t, db)

	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        saleID,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    100000,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// PHARMACY: the same Rx-flagged product is gated — adding it with no covering
// prescription is rejected. (The covering-Rx happy path is in rx_coverage_test.)
func TestPos_PharmacyMode_RxRequiresPrescription(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	seedStock(t, db, prodID, ownerID, 50)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 1,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// Non-Rx products sell identically in both modes — a quick guard that the
// pharmacy gate never touches ordinary products.
func TestPos_PharmacyMode_NonRxProductSellsFreely(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedProduct(t, db, "SOAP", "Soap bar", 500) // non-Rx
	seedStock(t, db, prodID, ownerID, 50)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 2,
	}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        saleID,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    100000,
	}))
	require.NoError(t, err)
}

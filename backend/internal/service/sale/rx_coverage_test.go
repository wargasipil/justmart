package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
)

// AddItem of an Rx-required product with no prescription attached is blocked.
func TestAddItem_RxRequiredWithoutPrescription(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db) // Rx gate only fires in pharmacy mode
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	seedStock(t, db, prodID, ownerID, 50)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 1,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// AddItem of an Rx-required product is allowed once a covering Rx is attached,
// and rejected when the quantity exceeds the prescribed remaining.
func TestAddItem_RxRequiredWithPrescription(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	seedStock(t, db, prodID, ownerID, 50)
	custID := seedCustomer(t, db, "Budi")
	rxID := seedPrescription(t, db, custID, ownerID, prodID, 10)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AttachPrescription(ctx, connect.NewRequest(&posifacev1.AttachPrescriptionRequest{
		SaleId: saleID, PrescriptionId: rxID,
	}))
	require.NoError(t, err)

	// Within prescribed (10) -> OK.
	_, err = svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 6,
	}))
	require.NoError(t, err)

	// 6 + 6 = 12 > prescribed 10 -> rejected.
	_, err = svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 6,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// CompleteSale accrues dispensed_qty against the attached prescription.
func TestCompleteSale_IncrementsRxDispensed(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	seedStock(t, db, prodID, ownerID, 50)
	custID := seedCustomer(t, db, "Budi")
	rxID := seedPrescription(t, db, custID, ownerID, prodID, 10)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AttachPrescription(ctx, connect.NewRequest(&posifacev1.AttachPrescriptionRequest{
		SaleId: saleID, PrescriptionId: rxID,
	}))
	require.NoError(t, err)
	_, err = svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 4,
	}))
	require.NoError(t, err)

	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        saleID,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    100000,
	}))
	require.NoError(t, err)

	var dispensed int32
	require.NoError(t, db.Table("prescription_items").
		Where("prescription_id = ? AND product_id = ?", rxID, prodID).
		Select("dispensed_qty").Scan(&dispensed).Error)
	require.Equal(t, int32(4), dispensed)
}

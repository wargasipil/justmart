package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
)

// Attaching a resep snapshots its biaya_jasa onto the sale and folds it into the
// order total; adding items adds on top.
func TestServiceFee_AttachSnapshotsAndIncludesInTotal(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	seedStock(t, db, prodID, ownerID, 50)
	custID := seedCustomer(t, db, "Budi")
	rxID := seedPrescription(t, db, custID, ownerID, prodID, 10)
	require.NoError(t, db.Table("prescriptions").Where("id = ?", rxID).Update("biaya_jasa", int64(5000)).Error)

	saleID := startDraft(t, svc, ctx)
	att, err := svc.AttachPrescription(ctx, connect.NewRequest(&posifacev1.AttachPrescriptionRequest{
		SaleId: saleID, PrescriptionId: rxID,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(5000), att.Msg.Sale.BiayaJasa)
	require.Equal(t, int64(5000), att.Msg.Sale.Total) // no items yet: 0 + fee

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 2,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(2000), add.Msg.Sale.Subtotal)        // 2 x 1000
	require.Equal(t, int64(7000), add.Msg.Sale.Total)           // subtotal + fee
	require.Equal(t, int64(5000), add.Msg.Sale.BiayaJasa)
}

// Detaching the resep clears the fee snapshot and recomputes the total.
func TestServiceFee_DetachResets(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	custID := seedCustomer(t, db, "Budi")
	rxID := seedPrescription(t, db, custID, ownerID, prodID, 10)
	require.NoError(t, db.Table("prescriptions").Where("id = ?", rxID).Update("biaya_jasa", int64(5000)).Error)

	saleID := startDraft(t, svc, ctx)
	_, err := svc.AttachPrescription(ctx, connect.NewRequest(&posifacev1.AttachPrescriptionRequest{
		SaleId: saleID, PrescriptionId: rxID,
	}))
	require.NoError(t, err)
	det, err := svc.DetachPrescription(ctx, connect.NewRequest(&posifacev1.DetachPrescriptionRequest{SaleId: saleID}))
	require.NoError(t, err)
	require.Equal(t, int64(0), det.Msg.Sale.BiayaJasa)
	require.Equal(t, int64(0), det.Msg.Sale.Total)
}

// SetServiceFee overrides the fee at POS and recomputes the total; negative rejected.
func TestSetServiceFee(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)
	saleID := startDraft(t, svc, ctx)

	resp, err := svc.SetServiceFee(ctx, connect.NewRequest(&posifacev1.SetServiceFeeRequest{
		SaleId: saleID, BiayaJasa: 8000,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(8000), resp.Msg.Sale.BiayaJasa)
	require.Equal(t, int64(8000), resp.Msg.Sale.Total)

	_, err = svc.SetServiceFee(ctx, connect.NewRequest(&posifacev1.SetServiceFeeRequest{
		SaleId: saleID, BiayaJasa: -1,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// CompleteSale carries the service fee into the finalized total.
func TestCompleteSale_IncludesServiceFee(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	seedStock(t, db, prodID, ownerID, 50)
	custID := seedCustomer(t, db, "Budi")
	rxID := seedPrescription(t, db, custID, ownerID, prodID, 10)
	require.NoError(t, db.Table("prescriptions").Where("id = ?", rxID).Update("biaya_jasa", int64(5000)).Error)

	saleID := startDraft(t, svc, ctx)
	_, err := svc.AttachPrescription(ctx, connect.NewRequest(&posifacev1.AttachPrescriptionRequest{SaleId: saleID, PrescriptionId: rxID}))
	require.NoError(t, err)
	_, err = svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 2}))
	require.NoError(t, err)

	// Underpaying the fee-inclusive total (7000) is rejected.
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 6000,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	res, err := svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 7000,
	}))
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_COMPLETED, res.Msg.Sale.Status)
	require.Equal(t, int64(7000), res.Msg.Sale.Total)
	require.Equal(t, int64(5000), res.Msg.Sale.BiayaJasa)
}

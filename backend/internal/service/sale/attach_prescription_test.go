package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
)

func TestAttachPrescription_HappyPathAutoFillsCustomer(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	custID := seedCustomer(t, db, "Budi")
	rxID := seedPrescription(t, db, custID, ownerID, prodID, 10)
	saleID := startDraft(t, svc, ctx)

	resp, err := svc.AttachPrescription(ctx, connect.NewRequest(&posifacev1.AttachPrescriptionRequest{
		SaleId:         saleID,
		PrescriptionId: rxID,
	}))
	require.NoError(t, err)
	require.Equal(t, rxID, resp.Msg.Sale.PrescriptionId)
	require.Equal(t, custID, resp.Msg.Sale.CustomerId) // auto-filled from the Rx
}

func TestAttachPrescription_CustomerMismatch(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	rxCust := seedCustomer(t, db, "Budi")
	otherCust := seedCustomer(t, db, "Citra")
	rxID := seedPrescription(t, db, rxCust, ownerID, prodID, 10)
	saleID := startDraft(t, svc, ctx)

	// Set the sale's customer to someone other than the Rx patient.
	_, err := svc.SetSaleCustomer(ctx, connect.NewRequest(&posifacev1.SetSaleCustomerRequest{
		SaleId: saleID, CustomerId: otherCust,
	}))
	require.NoError(t, err)

	_, err = svc.AttachPrescription(ctx, connect.NewRequest(&posifacev1.AttachPrescriptionRequest{
		SaleId: saleID, PrescriptionId: rxID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestAttachPrescription_VoidedRejected(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	custID := seedCustomer(t, db, "Budi")
	rxID := seedPrescription(t, db, custID, ownerID, prodID, 10)
	require.NoError(t, db.Table("prescriptions").Where("id = ?", rxID).Update("status", "VOIDED").Error)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.AttachPrescription(ctx, connect.NewRequest(&posifacev1.AttachPrescriptionRequest{
		SaleId: saleID, PrescriptionId: rxID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestAttachPrescription_NotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.AttachPrescription(ctx, connect.NewRequest(&posifacev1.AttachPrescriptionRequest{
		SaleId: saleID, PrescriptionId: "00000000-0000-0000-0000-0000000000ff",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

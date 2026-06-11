package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
)

func TestDetachPrescription_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	custID := seedCustomer(t, db, "Budi")
	rxID := seedPrescription(t, db, custID, ownerID, prodID, 10)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AttachPrescription(ctx, connect.NewRequest(&posifacev1.AttachPrescriptionRequest{
		SaleId: saleID, PrescriptionId: rxID,
	}))
	require.NoError(t, err)

	// No cart items yet -> detach is allowed.
	resp, err := svc.DetachPrescription(ctx, connect.NewRequest(&posifacev1.DetachPrescriptionRequest{
		SaleId: saleID,
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Sale.PrescriptionId)
}

func TestDetachPrescription_BlockedWithRxItemInCart(t *testing.T) {
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
		SaleId: saleID, ProductId: prodID, Qty: 2,
	}))
	require.NoError(t, err)

	// Rx-required item still in the cart -> detach blocked.
	_, err = svc.DetachPrescription(ctx, connect.NewRequest(&posifacev1.DetachPrescriptionRequest{
		SaleId: saleID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

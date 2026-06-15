package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
)

func TestRemoveItem_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	productID := seedProduct(t, db, "ri-sku-1", "Antasida", 800)
	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 2,
	}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id

	resp, err := svc.RemoveItem(ctx, connect.NewRequest(&posifacev1.RemoveItemRequest{
		SaleId: saleID, ItemId: itemID,
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Sale.Items)
	require.Equal(t, int64(0), resp.Msg.Sale.Subtotal)
	require.Equal(t, int64(0), resp.Msg.Sale.Total)
}

// In pharmacy mode, removing the Rx-required line clears the cart of Rx items so
// the prescription can then be detached (detach is blocked while such items
// remain). Exercises the remove ↔ Rx-gate interplay in pharmacy mode.
func TestRemoveItem_PharmacyMode_FreesRxCoverage(t *testing.T) {
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
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 2,
	}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id

	// While the Rx item is in the cart, detach is blocked.
	_, err = svc.DetachPrescription(ctx, connect.NewRequest(&posifacev1.DetachPrescriptionRequest{SaleId: saleID}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	// Remove the Rx line → no Rx-required items remain → detach now succeeds.
	_, err = svc.RemoveItem(ctx, connect.NewRequest(&posifacev1.RemoveItemRequest{
		SaleId: saleID, ItemId: itemID,
	}))
	require.NoError(t, err)
	det, err := svc.DetachPrescription(ctx, connect.NewRequest(&posifacev1.DetachPrescriptionRequest{SaleId: saleID}))
	require.NoError(t, err)
	require.Empty(t, det.Msg.Sale.PrescriptionId)
}

func TestRemoveItem_ItemNotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.RemoveItem(ctx, connect.NewRequest(&posifacev1.RemoveItemRequest{
		SaleId: saleID,
		ItemId: "00000000-0000-0000-0000-0000000000bb",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
)

func TestSetItemQuantity_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	productID := seedProduct(t, db, "siq-sku-1", "Cetirizine", 3000)
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 2,
	}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id

	resp, err := svc.SetItemQuantity(ctx, connect.NewRequest(&posifacev1.SetItemQuantityRequest{
		SaleId: saleID, ItemId: itemID, Qty: 5,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Sale.Items, 1)
	require.Equal(t, int32(5), resp.Msg.Sale.Items[0].Qty)
	require.Equal(t, int64(15000), resp.Msg.Sale.Items[0].LineTotal)
	require.Equal(t, int64(15000), resp.Msg.Sale.Total)
}

func TestSetItemQuantity_ZeroQty(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	productID := seedProduct(t, db, "siq-sku-2", "Loratadine", 2500)
	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 1,
	}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id

	_, err = svc.SetItemQuantity(ctx, connect.NewRequest(&posifacev1.SetItemQuantityRequest{
		SaleId: saleID, ItemId: itemID, Qty: 0,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSetItemQuantity_ItemNotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.SetItemQuantity(ctx, connect.NewRequest(&posifacev1.SetItemQuantityRequest{
		SaleId: saleID,
		ItemId: "00000000-0000-0000-0000-0000000000aa",
		Qty:    3,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// Pharmacy mode: SetItemQuantity re-checks Rx coverage — raising an Rx line above
// the prescribed remaining is rejected; within remaining is allowed.
func TestSetItemQuantity_PharmacyMode_RxCoverage(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
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

	// 12 > prescribed 10 -> rejected.
	_, err = svc.SetItemQuantity(ctx, connect.NewRequest(&posifacev1.SetItemQuantityRequest{
		SaleId: saleID, ItemId: itemID, Qty: 12,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	// 8 <= 10 -> allowed.
	_, err = svc.SetItemQuantity(ctx, connect.NewRequest(&posifacev1.SetItemQuantityRequest{
		SaleId: saleID, ItemId: itemID, Qty: 8,
	}))
	require.NoError(t, err)
}

// Retail mode: SetItemQuantity ignores the Rx flag — raising the qty on an
// Rx-flagged product with no prescription is allowed (gate is a no-op).
func TestSetItemQuantity_RetailMode_RxFlagIgnored(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	setRetailMode(t, db)
	prodID := seedRxProduct(t, db, "AMOX", "Amoxicillin", 1000)
	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 1,
	}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id

	_, err = svc.SetItemQuantity(ctx, connect.NewRequest(&posifacev1.SetItemQuantityRequest{
		SaleId: saleID, ItemId: itemID, Qty: 9,
	}))
	require.NoError(t, err)
}

package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
)

// A manual line discount overrides the auto product discount; clearing it
// reverts the line to the auto product discount.
func TestClearLineDiscount_RevertsToAuto(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "CL1", "Obat", 1000)
	seedProductDiscount(t, db, prodID, "PERCENT", false, 1000, 0, nil) // 10% auto
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 2}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id
	require.Equal(t, int64(200), add.Msg.Sale.Items[0].LineDiscount) // auto 10%

	// Manual override beats the auto discount.
	man, err := svc.SetLineDiscount(ctx, connect.NewRequest(&posifacev1.SetLineDiscountRequest{
		SaleId: saleID, ItemId: itemID, DiscountType: "FIXED", DiscountValue: 500,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(500), man.Msg.Sale.Items[0].LineDiscount)
	require.True(t, man.Msg.Sale.Items[0].DiscountManual)

	// Manual override survives a qty change (rescale stays manual).
	q, err := svc.SetItemQuantity(ctx, connect.NewRequest(&posifacev1.SetItemQuantityRequest{SaleId: saleID, ItemId: itemID, Qty: 3}))
	require.NoError(t, err)
	require.Equal(t, int64(500), q.Msg.Sale.Items[0].LineDiscount) // FIXED stays 500
	require.True(t, q.Msg.Sale.Items[0].DiscountManual)

	// Clearing reverts to the auto product discount (10% of 3000 = 300).
	clr, err := svc.ClearLineDiscount(ctx, connect.NewRequest(&posifacev1.ClearLineDiscountRequest{SaleId: saleID, ItemId: itemID}))
	require.NoError(t, err)
	require.Equal(t, int64(300), clr.Msg.Sale.Items[0].LineDiscount)
	require.False(t, clr.Msg.Sale.Items[0].DiscountManual)
}

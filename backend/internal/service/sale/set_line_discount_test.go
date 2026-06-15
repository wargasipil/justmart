package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
)

func TestSetLineDiscount_Fixed(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "P1", "Antasida", 1000)
	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 2, // gross 2000
	}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id

	resp, err := svc.SetLineDiscount(ctx, connect.NewRequest(&posifacev1.SetLineDiscountRequest{
		SaleId: saleID, ItemId: itemID, DiscountType: "FIXED", DiscountValue: 500,
	}))
	require.NoError(t, err)
	it := resp.Msg.Sale.Items[0]
	require.Equal(t, "FIXED", it.DiscountType)
	require.Equal(t, int64(500), it.DiscountValue)
	require.Equal(t, int64(500), it.LineDiscount) // resolved amount
	require.Equal(t, int64(1500), it.LineTotal)   // 2000 - 500
	require.Equal(t, int64(1500), resp.Msg.Sale.Subtotal)
	require.Equal(t, int64(1500), resp.Msg.Sale.Total)
}

func TestSetLineDiscount_Percent(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "P1", "Antasida", 1000)
	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 2, // gross 2000
	}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id

	// 10% = 1000 basis points → 200 off 2000.
	resp, err := svc.SetLineDiscount(ctx, connect.NewRequest(&posifacev1.SetLineDiscountRequest{
		SaleId: saleID, ItemId: itemID, DiscountType: "PERCENT", DiscountValue: 1000,
	}))
	require.NoError(t, err)
	it := resp.Msg.Sale.Items[0]
	require.Equal(t, "PERCENT", it.DiscountType)
	require.Equal(t, int64(1000), it.DiscountValue)
	require.Equal(t, int64(200), it.LineDiscount)
	require.Equal(t, int64(1800), it.LineTotal)
	require.Equal(t, int64(1800), resp.Msg.Sale.Total)
}

// PERCENT discount rounds half-up: 10% of 25 = 2.5 → 3.
func TestSetLineDiscount_PercentRoundsHalfUp(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "P1", "Antasida", 25)
	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 1, // gross 25
	}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id

	resp, err := svc.SetLineDiscount(ctx, connect.NewRequest(&posifacev1.SetLineDiscountRequest{
		SaleId: saleID, ItemId: itemID, DiscountType: "PERCENT", DiscountValue: 1000, // 10%
	}))
	require.NoError(t, err)
	require.Equal(t, int64(3), resp.Msg.Sale.Items[0].LineDiscount) // 2.5 → 3
	require.Equal(t, int64(22), resp.Msg.Sale.Items[0].LineTotal)
}

// A FIXED discount larger than the gross is clamped — line_total never negative.
func TestSetLineDiscount_ClampToGross(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "P1", "Antasida", 1000)
	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 2, // gross 2000
	}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id

	resp, err := svc.SetLineDiscount(ctx, connect.NewRequest(&posifacev1.SetLineDiscountRequest{
		SaleId: saleID, ItemId: itemID, DiscountType: "FIXED", DiscountValue: 999999,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(2000), resp.Msg.Sale.Items[0].LineDiscount) // clamped to gross
	require.Equal(t, int64(0), resp.Msg.Sale.Items[0].LineTotal)
	require.Equal(t, int64(0), resp.Msg.Sale.Total)
}

// A PERCENT line discount re-resolves off the new gross when qty changes.
func TestSetLineDiscount_PercentReResolvesOnQtyChange(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "P1", "Antasida", 1000)
	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 2, // gross 2000
	}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id

	_, err = svc.SetLineDiscount(ctx, connect.NewRequest(&posifacev1.SetLineDiscountRequest{
		SaleId: saleID, ItemId: itemID, DiscountType: "PERCENT", DiscountValue: 1000, // 10% → 200
	}))
	require.NoError(t, err)

	// Bump qty to 4 (gross 4000) → discount re-resolves to 400.
	resp, err := svc.SetItemQuantity(ctx, connect.NewRequest(&posifacev1.SetItemQuantityRequest{
		SaleId: saleID, ItemId: itemID, Qty: 4,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(400), resp.Msg.Sale.Items[0].LineDiscount)
	require.Equal(t, int64(3600), resp.Msg.Sale.Items[0].LineTotal)
	require.Equal(t, int64(3600), resp.Msg.Sale.Total)
}

func TestSetLineDiscount_InvalidInputs(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "P1", "Antasida", 1000)
	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 2,
	}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id

	cases := []struct {
		name  string
		typ   string
		value int64
	}{
		{"negative", "FIXED", -1},
		{"bad type", "WEIRD", 100},
		{"percent over 100", "PERCENT", 10001},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.SetLineDiscount(ctx, connect.NewRequest(&posifacev1.SetLineDiscountRequest{
				SaleId: saleID, ItemId: itemID, DiscountType: c.typ, DiscountValue: c.value,
			}))
			require.Error(t, err)
			require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
		})
	}
}

func TestSetLineDiscount_ItemNotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.SetLineDiscount(ctx, connect.NewRequest(&posifacev1.SetLineDiscountRequest{
		SaleId: saleID, ItemId: "00000000-0000-0000-0000-0000000000bb", DiscountType: "FIXED", DiscountValue: 100,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestSetLineDiscount_DraftOnly(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "P1", "Antasida", 1000)
	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 1,
	}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id

	_, err = svc.VoidSale(ctx, connect.NewRequest(&posifacev1.VoidSaleRequest{SaleId: saleID}))
	require.NoError(t, err)

	_, err = svc.SetLineDiscount(ctx, connect.NewRequest(&posifacev1.SetLineDiscountRequest{
		SaleId: saleID, ItemId: itemID, DiscountType: "FIXED", DiscountValue: 100,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

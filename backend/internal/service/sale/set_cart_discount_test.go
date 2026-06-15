package sale_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	salesvc "github.com/justmart/backend/internal/service/sale"
)

// addCart seeds a product (price 1000), starts a draft, and adds `qty` so the
// subtotal is qty×1000. Returns the saleID.
func addCart(t *testing.T, svc *salesvc.SaleService, ctx context.Context, db *gorm.DB, qty int32) string {
	t.Helper()
	prodID := seedProduct(t, db, "P1", "Antasida", 1000)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: qty,
	}))
	require.NoError(t, err)
	return saleID
}

func TestSetCartDiscount_Fixed(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	saleID := addCart(t, svc, ctx, db, 2) // subtotal 2000

	resp, err := svc.SetCartDiscount(ctx, connect.NewRequest(&posifacev1.SetCartDiscountRequest{
		SaleId: saleID, DiscountType: "FIXED", DiscountValue: 500,
	}))
	require.NoError(t, err)
	require.Equal(t, "FIXED", resp.Msg.Sale.CartDiscountType)
	require.Equal(t, int64(500), resp.Msg.Sale.CartDiscountValue)
	require.Equal(t, int64(500), resp.Msg.Sale.CartDiscount) // resolved
	require.Equal(t, int64(2000), resp.Msg.Sale.Subtotal)
	require.Equal(t, int64(1500), resp.Msg.Sale.Total)
}

func TestSetCartDiscount_Percent(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	saleID := addCart(t, svc, ctx, db, 2) // subtotal 2000

	resp, err := svc.SetCartDiscount(ctx, connect.NewRequest(&posifacev1.SetCartDiscountRequest{
		SaleId: saleID, DiscountType: "PERCENT", DiscountValue: 1000, // 10%
	}))
	require.NoError(t, err)
	require.Equal(t, "PERCENT", resp.Msg.Sale.CartDiscountType)
	require.Equal(t, int64(1000), resp.Msg.Sale.CartDiscountValue)
	require.Equal(t, int64(200), resp.Msg.Sale.CartDiscount)
	require.Equal(t, int64(1800), resp.Msg.Sale.Total)
}

// A PERCENT cart discount re-resolves off the subtotal as items change.
func TestSetCartDiscount_PercentReResolvesOnAdd(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "P1", "Antasida", 1000)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 2, // subtotal 2000
	}))
	require.NoError(t, err)

	_, err = svc.SetCartDiscount(ctx, connect.NewRequest(&posifacev1.SetCartDiscountRequest{
		SaleId: saleID, DiscountType: "PERCENT", DiscountValue: 1000, // 10% → 200
	}))
	require.NoError(t, err)

	// Add 2 more (subtotal 4000) → cart discount re-resolves to 400.
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 2,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(4000), add.Msg.Sale.Subtotal)
	require.Equal(t, int64(400), add.Msg.Sale.CartDiscount)
	require.Equal(t, int64(3600), add.Msg.Sale.Total)
}

// total = subtotal − cart_discount + biaya_jasa. The service fee is added after
// the discount (discount applies to the items subtotal only).
func TestSetCartDiscount_WithServiceFee(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	saleID := addCart(t, svc, ctx, db, 2) // subtotal 2000

	_, err := svc.SetServiceFee(ctx, connect.NewRequest(&posifacev1.SetServiceFeeRequest{
		SaleId: saleID, BiayaJasa: 5000,
	}))
	require.NoError(t, err)

	resp, err := svc.SetCartDiscount(ctx, connect.NewRequest(&posifacev1.SetCartDiscountRequest{
		SaleId: saleID, DiscountType: "FIXED", DiscountValue: 500,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(500), resp.Msg.Sale.CartDiscount)
	require.Equal(t, int64(5000), resp.Msg.Sale.BiayaJasa)
	require.Equal(t, int64(6500), resp.Msg.Sale.Total) // 2000 - 500 + 5000
}

func TestSetCartDiscount_ClampToSubtotal(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	saleID := addCart(t, svc, ctx, db, 2) // subtotal 2000

	resp, err := svc.SetCartDiscount(ctx, connect.NewRequest(&posifacev1.SetCartDiscountRequest{
		SaleId: saleID, DiscountType: "FIXED", DiscountValue: 999999,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(2000), resp.Msg.Sale.CartDiscount) // clamped to subtotal
	require.Equal(t, int64(0), resp.Msg.Sale.Total)
}

func TestSetCartDiscount_InvalidInputs(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	saleID := addCart(t, svc, ctx, db, 2)

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
			_, err := svc.SetCartDiscount(ctx, connect.NewRequest(&posifacev1.SetCartDiscountRequest{
				SaleId: saleID, DiscountType: c.typ, DiscountValue: c.value,
			}))
			require.Error(t, err)
			require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
		})
	}
}

func TestSetCartDiscount_DraftOnly(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	saleID := addCart(t, svc, ctx, db, 1)

	_, err := svc.VoidSale(ctx, connect.NewRequest(&posifacev1.VoidSaleRequest{SaleId: saleID}))
	require.NoError(t, err)

	_, err = svc.SetCartDiscount(ctx, connect.NewRequest(&posifacev1.SetCartDiscountRequest{
		SaleId: saleID, DiscountType: "FIXED", DiscountValue: 100,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

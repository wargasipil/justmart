package sale_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
)

func TestCompleteSale_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "cs-sku-1", "Paracetamol", 2000)
	seedStock(t, db, productID, ownerID, 50)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 3,
	}))
	require.NoError(t, err)

	resp, err := svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        saleID,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    10000,
	}))
	require.NoError(t, err)
	sale := resp.Msg.Sale
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_COMPLETED, sale.Status)
	require.NotEmpty(t, sale.SaleNo)        // INV-YYYY-NNNN assigned
	require.Equal(t, int64(6000), sale.Total)
	require.Equal(t, int64(10000), sale.PaidAmount)
	require.Equal(t, posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, sale.PaymentSource)
	require.NotZero(t, sale.CompletedAt)
}

// A sale with a per-line discount + a cart discount completes on the discounted
// total, and the cash guard is driven by that discounted total.
func TestCompleteSale_WithDiscounts(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "cs-disc", "Vitamin C", 1000)
	seedStock(t, db, productID, ownerID, 50)
	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 2, // gross 2000
	}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id

	// Line discount: FIXED 200 → line_total 1800, subtotal 1800.
	_, err = svc.SetLineDiscount(ctx, connect.NewRequest(&posifacev1.SetLineDiscountRequest{
		SaleId: saleID, ItemId: itemID, DiscountType: "FIXED", DiscountValue: 200,
	}))
	require.NoError(t, err)
	// Cart discount: PERCENT 10% of 1800 → 180 → total 1620.
	cd, err := svc.SetCartDiscount(ctx, connect.NewRequest(&posifacev1.SetCartDiscountRequest{
		SaleId: saleID, DiscountType: "PERCENT", DiscountValue: 1000,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(1800), cd.Msg.Sale.Subtotal)
	require.Equal(t, int64(180), cd.Msg.Sale.CartDiscount)
	require.Equal(t, int64(1620), cd.Msg.Sale.Total)

	// Underpaying the discounted total is rejected.
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 1000,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	// Paying the discounted total completes.
	res, err := svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 2000,
	}))
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_COMPLETED, res.Msg.Sale.Status)
	require.Equal(t, int64(1620), res.Msg.Sale.Total)
	require.Equal(t, int64(180), res.Msg.Sale.CartDiscount)
	require.Equal(t, int64(200), res.Msg.Sale.Items[0].LineDiscount)
}

func TestCompleteSale_EmptyCart(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        saleID,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    1000,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestCompleteSale_InsufficientStock(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "cs-sku-2", "Amoxicillin", 1500)
	seedStock(t, db, productID, ownerID, 1) // only 1 in stock
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 5,
	}))
	require.NoError(t, err)

	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        saleID,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    100000,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestCompleteSale_CashUnderpaid(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "cs-sku-3", "Vitamin C", 1000)
	seedStock(t, db, productID, ownerID, 50)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 5,
	}))
	require.NoError(t, err)

	// total is 5000, pay only 1000 in cash -> InvalidArgument.
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        saleID,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    1000,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCompleteSale_MissingPaymentSource(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID,
		// PaymentSource unspecified -> InvalidArgument.
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCompleteSale_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newSaleSvc(t)

	_, err := svc.CompleteSale(context.Background(), connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        "00000000-0000-0000-0000-0000000000dd",
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    1000,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

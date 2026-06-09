package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
)

func TestDiscardSale_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	productID := seedProduct(t, db, "dis-sku-1", "Paracetamol", 2000)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 2,
	}))
	require.NoError(t, err)

	_, err = svc.DiscardSale(ctx, connect.NewRequest(&posifacev1.DiscardSaleRequest{
		SaleId: saleID,
	}))
	require.NoError(t, err)

	// The draft is gone: GetSale now returns NotFound.
	_, err = svc.GetSale(ctx, connect.NewRequest(&posifacev1.GetSaleRequest{Id: saleID}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDiscardSale_NotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)

	_, err := svc.DiscardSale(ctx, connect.NewRequest(&posifacev1.DiscardSaleRequest{
		SaleId: "00000000-0000-0000-0000-0000000000a8",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDiscardSale_NonDraft(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "dis-sku-2", "Amoxicillin", 1500)
	seedStock(t, db, productID, ownerID, 10)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 1,
	}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 5000,
	}))
	require.NoError(t, err)

	// A COMPLETED sale cannot be discarded.
	_, err = svc.DiscardSale(ctx, connect.NewRequest(&posifacev1.DiscardSaleRequest{SaleId: saleID}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

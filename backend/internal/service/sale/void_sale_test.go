package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
)

func TestVoidSale_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)
	saleID := startDraft(t, svc, ctx)

	resp, err := svc.VoidSale(ctx, connect.NewRequest(&posifacev1.VoidSaleRequest{
		SaleId: saleID,
	}))
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_VOIDED, resp.Msg.Sale.Status)
}

func TestVoidSale_NotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)

	_, err := svc.VoidSale(ctx, connect.NewRequest(&posifacev1.VoidSaleRequest{
		SaleId: "00000000-0000-0000-0000-0000000000a9",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestVoidSale_NonDraft(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "vs-sku-1", "Paracetamol", 2000)
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

	// A COMPLETED sale cannot be voided.
	_, err = svc.VoidSale(ctx, connect.NewRequest(&posifacev1.VoidSaleRequest{SaleId: saleID}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

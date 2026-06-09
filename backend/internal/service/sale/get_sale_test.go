package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
)

func TestGetSale_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	productID := seedProduct(t, db, "gs-sku-1", "Paracetamol", 2000)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 2,
	}))
	require.NoError(t, err)

	resp, err := svc.GetSale(ctx, connect.NewRequest(&posifacev1.GetSaleRequest{Id: saleID}))
	require.NoError(t, err)
	require.Equal(t, saleID, resp.Msg.Sale.Id)
	require.Len(t, resp.Msg.Sale.Items, 1)
	require.Equal(t, productID, resp.Msg.Sale.Items[0].ProductId)
}

func TestGetSale_NotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)

	_, err := svc.GetSale(ctx, connect.NewRequest(&posifacev1.GetSaleRequest{
		Id: "00000000-0000-0000-0000-0000000000a7",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSale_EmptyID(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)

	_, err := svc.GetSale(ctx, connect.NewRequest(&posifacev1.GetSaleRequest{Id: ""}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

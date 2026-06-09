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

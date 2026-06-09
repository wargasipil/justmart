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

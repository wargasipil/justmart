package productrecipe_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

// Update edits the quantity and note in place.
func TestUpdateProductRecipeItem_EditsQtyAndNote(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newRecipeEnv(t)
	dish := seedProductOfKind(t, db, "MENU-UPD", compositeKind)
	rice := seedProductOfKind(t, db, "ING-RICE", stockedKind)
	item := addLine(t, svc, ctx, dish, rice, 200)

	resp, err := svc.UpdateProductRecipeItem(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRecipeItemRequest{
		Id: item.Id, QtyBase: 250, Note: "  cooked weight  ",
	}))
	require.NoError(t, err)
	require.Equal(t, int64(250), resp.Msg.Item.QtyBase)
	require.Equal(t, "cooked weight", resp.Msg.Item.Note)

	// Persisted, not just echoed.
	list, err := svc.ListProductRecipeItems(ctx, connect.NewRequest(&inventoryifacev1.ListProductRecipeItemsRequest{
		ProductId: dish,
	}))
	require.NoError(t, err)
	require.Len(t, list.Msg.Items, 1)
	require.Equal(t, int64(250), list.Msg.Items[0].QtyBase)
}

// The same qty rule as create — an update can't sneak a zero or negative in.
func TestUpdateProductRecipeItem_RejectsNonPositiveQty(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newRecipeEnv(t)
	dish := seedProductOfKind(t, db, "MENU-UPDQ", compositeKind)
	rice := seedProductOfKind(t, db, "ING-RICE", stockedKind)
	item := addLine(t, svc, ctx, dish, rice, 200)

	_, err := svc.UpdateProductRecipeItem(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRecipeItemRequest{
		Id: item.Id, QtyBase: 0,
	}))
	requireToken(t, err, connect.CodeInvalidArgument, "product_recipe.qty_invalid")
}

func TestUpdateProductRecipeItem_NotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newRecipeEnv(t)

	_, err := svc.UpdateProductRecipeItem(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRecipeItemRequest{
		Id: "00000000-0000-0000-0000-0000000000ff", QtyBase: 5,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// Delete removes the line and frees the component so it can be re-added.
func TestDeleteProductRecipeItem_RemovesAndFreesComponent(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newRecipeEnv(t)
	dish := seedProductOfKind(t, db, "MENU-DEL", compositeKind)
	rice := seedProductOfKind(t, db, "ING-RICE", stockedKind)
	item := addLine(t, svc, ctx, dish, rice, 200)

	_, err := svc.DeleteProductRecipeItem(ctx, connect.NewRequest(&inventoryifacev1.DeleteProductRecipeItemRequest{
		Id: item.Id,
	}))
	require.NoError(t, err)

	list, err := svc.ListProductRecipeItems(ctx, connect.NewRequest(&inventoryifacev1.ListProductRecipeItemsRequest{
		ProductId: dish,
	}))
	require.NoError(t, err)
	require.Empty(t, list.Msg.Items)

	// The unique index no longer blocks the component.
	addLine(t, svc, ctx, dish, rice, 300)
}

func TestDeleteProductRecipeItem_NotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newRecipeEnv(t)

	_, err := svc.DeleteProductRecipeItem(ctx, connect.NewRequest(&inventoryifacev1.DeleteProductRecipeItemRequest{
		Id: "00000000-0000-0000-0000-0000000000ff",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

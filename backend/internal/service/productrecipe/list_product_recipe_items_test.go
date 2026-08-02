package productrecipe_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

// The list enriches each line with the component's display fields and its
// on-hand in the caller's active warehouse, and reports how many portions the
// current stock can build.
func TestListProductRecipeItems_EnrichesAndComputesBuildable(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newRecipeEnv(t)
	dish := seedProductOfKind(t, db, "MENU-NASGOR", compositeKind)
	rice := seedProductOfKind(t, db, "ING-RICE", stockedKind)
	egg := seedProductOfKind(t, db, "ING-EGG", stockedKind)

	addLine(t, svc, ctx, dish, rice, 200) // 200 g per portion
	addLine(t, svc, ctx, dish, egg, 1)    // 1 egg per portion

	seedComponentStock(t, db, rice, ownerID, 1000) // 5 portions' worth
	seedComponentStock(t, db, egg, ownerID, 3)     // 3 portions' worth <- the binding constraint

	resp, err := svc.ListProductRecipeItems(ctx, connect.NewRequest(&inventoryifacev1.ListProductRecipeItemsRequest{
		ProductId: dish,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(2), resp.Msg.Total)
	require.Len(t, resp.Msg.Items, 2)

	byComponent := map[string]*inventoryifacev1.ProductRecipeItem{}
	for _, it := range resp.Msg.Items {
		byComponent[it.ComponentProductId] = it
	}
	require.Equal(t, "ING-RICE", byComponent[rice].ComponentSku)
	require.Equal(t, "ING-RICE Item", byComponent[rice].ComponentName)
	require.Equal(t, "g", byComponent[rice].ComponentUnit)
	require.Equal(t, int64(1000), byComponent[rice].ComponentReady)
	require.Equal(t, int64(3), byComponent[egg].ComponentReady)

	// Bounded by whichever ingredient runs out first, not by the sum or the
	// average: 3 eggs => 3 portions even though there is rice for 5.
	require.Equal(t, int64(3), resp.Msg.Buildable)
}

// One empty jar makes the dish unbuildable, however much of everything else is
// in the store.
func TestListProductRecipeItems_BuildableZeroWhenAComponentIsOut(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newRecipeEnv(t)
	dish := seedProductOfKind(t, db, "MENU-2", compositeKind)
	rice := seedProductOfKind(t, db, "ING-RICE", stockedKind)
	egg := seedProductOfKind(t, db, "ING-EGG", stockedKind)
	addLine(t, svc, ctx, dish, rice, 200)
	addLine(t, svc, ctx, dish, egg, 1)
	seedComponentStock(t, db, rice, ownerID, 10_000)
	// egg: no stock at all

	resp, err := svc.ListProductRecipeItems(ctx, connect.NewRequest(&inventoryifacev1.ListProductRecipeItemsRequest{
		ProductId: dish,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(0), resp.Msg.Buildable)
}

// "No recipe yet" is a different fact from "out of an ingredient" — it reports
// -1 so the UI can say unconfigured instead of out-of-stock.
func TestListProductRecipeItems_EmptyRecipeReportsUnbounded(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newRecipeEnv(t)
	dish := seedProductOfKind(t, db, "MENU-EMPTY", compositeKind)

	resp, err := svc.ListProductRecipeItems(ctx, connect.NewRequest(&inventoryifacev1.ListProductRecipeItemsRequest{
		ProductId: dish,
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Items)
	require.Equal(t, int32(0), resp.Msg.Total)
	require.Equal(t, int64(-1), resp.Msg.Buildable)
}

// Paging returns a window while `total` stays the unpaged count, and `buildable`
// is computed over the WHOLE recipe — a page-local minimum would promise more
// portions than the kitchen can cook.
func TestListProductRecipeItems_PaginatesButBuildableSpansWholeRecipe(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newRecipeEnv(t)
	dish := seedProductOfKind(t, db, "MENU-PAGE", compositeKind)
	rice := seedProductOfKind(t, db, "ING-RICE", stockedKind)
	egg := seedProductOfKind(t, db, "ING-EGG", stockedKind)
	oil := seedProductOfKind(t, db, "ING-OIL", stockedKind)
	addLine(t, svc, ctx, dish, rice, 100)
	addLine(t, svc, ctx, dish, egg, 1)
	addLine(t, svc, ctx, dish, oil, 10)
	seedComponentStock(t, db, rice, ownerID, 10_000) // 100 portions
	seedComponentStock(t, db, egg, ownerID, 50)      // 50 portions
	seedComponentStock(t, db, oil, ownerID, 20)      // 2 portions <- binding, and on page 2

	resp, err := svc.ListProductRecipeItems(ctx, connect.NewRequest(&inventoryifacev1.ListProductRecipeItemsRequest{
		ProductId: dish, Limit: 2, Offset: 0,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 2)
	require.Equal(t, int32(3), resp.Msg.Total)
	require.Equal(t, int64(2), resp.Msg.Buildable)
}

func TestListProductRecipeItems_RequiresProductID(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newRecipeEnv(t)

	_, err := svc.ListProductRecipeItems(ctx, connect.NewRequest(&inventoryifacev1.ListProductRecipeItemsRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

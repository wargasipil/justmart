package productrecipe_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

// Happy path: a COMPOSITE parent takes STOCKED components.
func TestCreateProductRecipeItem_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newRecipeEnv(t)
	dish := seedProductOfKind(t, db, "MENU-NASGOR", compositeKind)
	rice := seedProductOfKind(t, db, "ING-RICE", stockedKind)

	item := addLine(t, svc, ctx, dish, rice, 200)
	require.NotEmpty(t, item.Id)
	require.Equal(t, dish, item.ProductId)
	require.Equal(t, rice, item.ComponentProductId)
	require.Equal(t, int64(200), item.QtyBase)
}

// A recipe only means something on a COMPOSITE product: on a STOCKED one the
// sale consumes its own batches and never consults the recipe, so accepting the
// line would let an owner fill in ingredients that are never deducted.
func TestCreateProductRecipeItem_RejectsNonCompositeParent(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newRecipeEnv(t)
	plain := seedProductOfKind(t, db, "PLAIN-1", stockedKind)
	rice := seedProductOfKind(t, db, "ING-RICE", stockedKind)

	_, err := svc.CreateProductRecipeItem(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRecipeItemRequest{
		ProductId: plain, ComponentProductId: rice, QtyBase: 100,
	}))
	requireToken(t, err, connect.CodeFailedPrecondition, "product_recipe.parent_not_composite")
}

// Components must be STOCKED. This is what makes nested recipes — and with them
// every possible cycle — unrepresentable, so the sale-time explosion is always
// exactly one level deep.
func TestCreateProductRecipeItem_RejectsNonStockedComponent(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newRecipeEnv(t)
	dish := seedProductOfKind(t, db, "MENU-1", compositeKind)
	sauce := seedProductOfKind(t, db, "SUB-SAUCE", compositeKind)
	fee := seedProductOfKind(t, db, "SVC-FEE", serviceKind)

	_, err := svc.CreateProductRecipeItem(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRecipeItemRequest{
		ProductId: dish, ComponentProductId: sauce, QtyBase: 1,
	}))
	requireToken(t, err, connect.CodeFailedPrecondition, "product_recipe.component_not_stocked")

	_, err = svc.CreateProductRecipeItem(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRecipeItemRequest{
		ProductId: dish, ComponentProductId: fee, QtyBase: 1,
	}))
	requireToken(t, err, connect.CodeFailedPrecondition, "product_recipe.component_not_stocked")
}

// A product can't be its own ingredient — the cheap half of cycle protection,
// backed by a DB CHECK.
func TestCreateProductRecipeItem_RejectsSelfReference(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newRecipeEnv(t)
	dish := seedProductOfKind(t, db, "MENU-SELF", compositeKind)

	_, err := svc.CreateProductRecipeItem(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRecipeItemRequest{
		ProductId: dish, ComponentProductId: dish, QtyBase: 1,
	}))
	requireToken(t, err, connect.CodeInvalidArgument, "product_recipe.component_self")
}

// An archived ingredient can't be added — it would resolve to a product the shop
// no longer stocks.
func TestCreateProductRecipeItem_RejectsArchivedComponent(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newRecipeEnv(t)
	dish := seedProductOfKind(t, db, "MENU-ARCH", compositeKind)
	gone := seedProductOfKind(t, db, "ING-GONE", stockedKind)
	require.NoError(t, db.Table("products").Where("id = ?", gone).Update("active", false).Error)

	_, err := svc.CreateProductRecipeItem(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRecipeItemRequest{
		ProductId: dish, ComponentProductId: gone, QtyBase: 1,
	}))
	requireToken(t, err, connect.CodeFailedPrecondition, "product_recipe.component_archived")
}

// qty_base must be positive: a zero line would consume nothing and a negative one
// would ADD stock on every sale.
func TestCreateProductRecipeItem_RejectsNonPositiveQty(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newRecipeEnv(t)
	dish := seedProductOfKind(t, db, "MENU-QTY", compositeKind)
	rice := seedProductOfKind(t, db, "ING-RICE", stockedKind)

	for _, q := range []int64{0, -5} {
		_, err := svc.CreateProductRecipeItem(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRecipeItemRequest{
			ProductId: dish, ComponentProductId: rice, QtyBase: q,
		}))
		requireToken(t, err, connect.CodeInvalidArgument, "product_recipe.qty_invalid")
	}
}

// One line per ingredient — to use more of it, raise qty_base. A second line for
// the same component would double-deduct in a way the editor can't show.
func TestCreateProductRecipeItem_RejectsDuplicateComponent(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newRecipeEnv(t)
	dish := seedProductOfKind(t, db, "MENU-DUP", compositeKind)
	rice := seedProductOfKind(t, db, "ING-RICE", stockedKind)
	addLine(t, svc, ctx, dish, rice, 200)

	_, err := svc.CreateProductRecipeItem(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRecipeItemRequest{
		ProductId: dish, ComponentProductId: rice, QtyBase: 50,
	}))
	requireToken(t, err, connect.CodeAlreadyExists, "product_recipe.component_taken")
}

package product_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	productsvc "github.com/justmart/backend/internal/service/product"
	"github.com/justmart/backend/internal/service/servicetest"
)

func newKindEnv(t *testing.T) (*productsvc.ProductService, context.Context, *gorm.DB, string) {
	t.Helper()
	db, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, db, cfg)
	return productsvc.NewProductService(db), servicetest.OwnerCtx(context.Background(), ownerID), db, ownerID
}

// A product created without a kind is STOCKED — the pre-migration default. This
// is what keeps every existing caller (and the CSV importer) on exactly the old
// behaviour.
func TestCreateProduct_DefaultsToStockedKind(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newKindEnv(t)

	resp, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: "SKU-KIND-DEF", Name: "Plain", Unit: "pcs", UnitPrice: 1000,
	}))
	require.NoError(t, err)
	require.Equal(t, inventoryifacev1.ProductKind_PRODUCT_KIND_STOCKED, resp.Msg.Product.Kind)
}

// COMPOSITE and SERVICE round-trip through create and read.
func TestCreateProduct_PersistsKind(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newKindEnv(t)

	for _, tc := range []struct {
		sku  string
		kind inventoryifacev1.ProductKind
	}{
		{"SKU-KIND-COMP", inventoryifacev1.ProductKind_PRODUCT_KIND_COMPOSITE},
		{"SKU-KIND-SVC", inventoryifacev1.ProductKind_PRODUCT_KIND_SERVICE},
	} {
		created, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
			Sku: tc.sku, Name: tc.sku, Unit: "pcs", UnitPrice: 1000, Kind: tc.kind,
		}))
		require.NoError(t, err)
		require.Equal(t, tc.kind, created.Msg.Product.Kind)

		got, err := svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{
			Id: created.Msg.Product.Id,
		}))
		require.NoError(t, err)
		require.Equal(t, tc.kind, got.Msg.Product.Kind)
	}
}

// Switching a COMPOSITE back to STOCKED while recipe lines still exist is
// refused: the rows would survive but stop being consulted, so the item would go
// on selling while deducting nothing from its ingredients.
func TestUpdateProduct_RejectsLeavingCompositeWithRecipeLines(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newKindEnv(t)

	dish, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: "SKU-DISH", Name: "Nasi Goreng", Unit: "porsi", UnitPrice: 20000,
		Kind: inventoryifacev1.ProductKind_PRODUCT_KIND_COMPOSITE,
	}))
	require.NoError(t, err)
	rice, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: "SKU-RICE", Name: "Beras", Unit: "g", UnitPrice: 0,
	}))
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.ProductRecipeItem{
		ProductID: dish.Msg.Product.Id, ComponentProductID: rice.Msg.Product.Id, QtyBase: 200,
	}).Error)

	_, err = svc.UpdateProduct(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
		Id: dish.Msg.Product.Id, Sku: "SKU-DISH", Name: "Nasi Goreng", Unit: "porsi", UnitPrice: 20000,
		Kind: inventoryifacev1.ProductKind_PRODUCT_KIND_STOCKED,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	require.Equal(t, "product.kind_has_recipe", ce.Message())

	// Clearing the recipe first makes the same change legal.
	require.NoError(t, db.Where("product_id = ?", dish.Msg.Product.Id).
		Delete(&model.ProductRecipeItem{}).Error)
	_, err = svc.UpdateProduct(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
		Id: dish.Msg.Product.Id, Sku: "SKU-DISH", Name: "Nasi Goreng", Unit: "porsi", UnitPrice: 20000,
		Kind: inventoryifacev1.ProductKind_PRODUCT_KIND_STOCKED,
	}))
	require.NoError(t, err)
}

// A composite holds no batches, so the ordinary ready-stock join leaves it at 0.
// The catalog must instead report how many portions its ingredients can build —
// bounded by whichever ingredient runs out first.
func TestGetProduct_CompositeReadyStockIsBuildablePortions(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newKindEnv(t)
	whID := defaultWarehouseID(t, db)

	dish, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: "SKU-DISH-2", Name: "Nasi Goreng", Unit: "porsi", UnitPrice: 20000,
		Kind: inventoryifacev1.ProductKind_PRODUCT_KIND_COMPOSITE,
	}))
	require.NoError(t, err)
	rice, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: "SKU-RICE-2", Name: "Beras", Unit: "g", UnitPrice: 0,
	}))
	require.NoError(t, err)
	egg, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: "SKU-EGG-2", Name: "Telur", Unit: "butir", UnitPrice: 0,
	}))
	require.NoError(t, err)

	require.NoError(t, db.Create(&model.ProductRecipeItem{
		ProductID: dish.Msg.Product.Id, ComponentProductID: rice.Msg.Product.Id, QtyBase: 200,
	}).Error)
	require.NoError(t, db.Create(&model.ProductRecipeItem{
		ProductID: dish.Msg.Product.Id, ComponentProductID: egg.Msg.Product.Id, QtyBase: 1,
	}).Error)

	seedBatchWithStock(t, db, rice.Msg.Product.Id, whID, ownerID, 1000, 10) // 5 portions
	seedBatchWithStock(t, db, egg.Msg.Product.Id, whID, ownerID, 4, 2000)   // 4 portions

	got, err := svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{
		Id: dish.Msg.Product.Id,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(4), got.Msg.Product.ReadyStock)

	// The recipe hydrates alongside units/tiers, with component display fields,
	// so POS can name what is missing without a second RPC.
	require.Len(t, got.Msg.Product.Recipe, 2)
	names := map[string]bool{}
	for _, r := range got.Msg.Product.Recipe {
		names[r.ComponentName] = true
	}
	require.True(t, names["Beras"])
	require.True(t, names["Telur"])
}

// A composite with no recipe reads 0, not "unbounded": nothing is defined for it
// to consume, so it is misconfigured, and 0 is the reading that sends someone to
// the recipe card.
func TestGetProduct_CompositeWithoutRecipeReadsZero(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newKindEnv(t)

	dish, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: "SKU-DISH-3", Name: "Menu Baru", Unit: "porsi", UnitPrice: 20000,
		Kind: inventoryifacev1.ProductKind_PRODUCT_KIND_COMPOSITE,
	}))
	require.NoError(t, err)

	got, err := svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{
		Id: dish.Msg.Product.Id,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(0), got.Msg.Product.ReadyStock)
	require.Empty(t, got.Msg.Product.Recipe)
}

// The overlay must not disturb ordinary products sharing the page — a regression
// here would silently zero the whole catalog's stock.
func TestListProducts_StockedProductsUnaffectedByCompositeOverlay(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newKindEnv(t)
	whID := defaultWarehouseID(t, db)

	plain, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: "SKU-PLAIN", Name: "Teh Botol", Unit: "pcs", UnitPrice: 5000,
	}))
	require.NoError(t, err)
	seedBatchWithStock(t, db, plain.Msg.Product.Id, whID, ownerID, 42, 3000)

	_, err = svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: "SKU-DISH-4", Name: "Nasi Goreng", Unit: "porsi", UnitPrice: 20000,
		Kind: inventoryifacev1.ProductKind_PRODUCT_KIND_COMPOSITE,
	}))
	require.NoError(t, err)

	list, err := svc.ListProducts(ctx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{}))
	require.NoError(t, err)
	var found bool
	for _, p := range list.Msg.Products {
		if p.Id == plain.Msg.Product.Id {
			found = true
			require.Equal(t, int64(42), p.ReadyStock)
		}
	}
	require.True(t, found)
}

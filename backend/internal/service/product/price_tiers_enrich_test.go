package product_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	productsvc "github.com/justmart/backend/internal/service/product"
	"github.com/justmart/backend/internal/service/servicetest"
)

// newProductSvc is the common setup: fresh DB, bootstrap owner, product service,
// OWNER ctx (CreateProduct FKs caller.UserID into product_prices.changed_by).
func newProductSvc(t *testing.T) (*productsvc.ProductService, context.Context, *gorm.DB, string) {
	t.Helper()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	return productsvc.NewProductService(gormDB), servicetest.OwnerCtx(context.Background(), ownerID), gormDB, ownerID
}

// seedTier adds a grosir rung directly to a product unit.
func seedTier(t *testing.T, db *gorm.DB, productID, unitID, unitName string, factor int64, minQty int32, price int64) {
	t.Helper()
	require.NoError(t, db.Create(&model.ProductPriceTier{
		ProductID: productID, ProductUnitID: unitID,
		UnitName: unitName, UnitFactor: factor, MinQty: minQty, Price: price,
	}).Error)
}

func baseUnit(t *testing.T, db *gorm.DB, productID string) model.ProductUnit {
	t.Helper()
	var u model.ProductUnit
	require.NoError(t, db.Where("product_id = ? AND is_base", productID).First(&u).Error)
	return u
}

func TestGetProduct_HydratesPriceTiers(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newProductSvc(t)
	prodID := seedProduct(t, svc, ctx, "PTE-1", "Kopi", 10000)
	base := baseUnit(t, db, prodID)
	seedTier(t, db, prodID, base.ID, base.Name, 1, 60, 8000)
	seedTier(t, db, prodID, base.ID, base.Name, 1, 12, 8500)

	resp, err := svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: prodID}))
	require.NoError(t, err)
	tiers := resp.Msg.Product.PriceTiers
	require.Len(t, tiers, 2)
	require.Equal(t, int32(12), tiers[0].MinQty, "ascending by threshold")
	require.Equal(t, int32(60), tiers[1].MinQty)
	require.Equal(t, int64(8500), tiers[0].Price)
}

// POS reads the catalog through ListProducts, so tiers MUST hydrate there — a
// Get-only hydration would leave every POS wholesale hint silently blank.
func TestListProducts_HydratesPriceTiersPerProduct(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newProductSvc(t)
	prodA := seedProduct(t, svc, ctx, "PTE-2a", "Gula", 10000)
	prodB := seedProduct(t, svc, ctx, "PTE-2b", "Teh", 5000)
	seedTier(t, db, prodA, baseUnit(t, db, prodA).ID, "tablet", 1, 12, 8500)
	seedTier(t, db, prodB, baseUnit(t, db, prodB).ID, "tablet", 1, 20, 4500)

	resp, err := svc.ListProducts(ctx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{}))
	require.NoError(t, err)
	byID := map[string]*inventoryifacev1.Product{}
	for _, p := range resp.Msg.Products {
		byID[p.Id] = p
	}
	require.Len(t, byID[prodA].PriceTiers, 1, "each product gets its own tiers (batch, no N+1)")
	require.Equal(t, int32(12), byID[prodA].PriceTiers[0].MinQty)
	require.Len(t, byID[prodB].PriceTiers, 1)
	require.Equal(t, int32(20), byID[prodB].PriceTiers[0].MinQty)
}

// A product with no ladder reports none rather than inheriting another's.
func TestListProducts_NoTiersIsEmpty(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newProductSvc(t)
	withTiers := seedProduct(t, svc, ctx, "PTE-3a", "Beras", 10000)
	plain := seedProduct(t, svc, ctx, "PTE-3b", "Sabun", 3000)
	seedTier(t, db, withTiers, baseUnit(t, db, withTiers).ID, "tablet", 1, 12, 8500)

	resp, err := svc.ListProducts(ctx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{}))
	require.NoError(t, err)
	for _, p := range resp.Msg.Products {
		if p.Id == plain {
			require.Empty(t, p.PriceTiers)
		}
	}
}

// Tiers of an archived unit are hidden, matching attachUnits — otherwise POS
// would hint a wholesale price for a unit it can no longer sell.
func TestGetProduct_ArchivedUnitTiersHidden(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newProductSvc(t)
	prodID := seedProduct(t, svc, ctx, "PTE-4", "Minyak", 10000)
	box := model.ProductUnit{
		ProductID: prodID, Name: "box", Factor: 12,
		SellPrice: 110000, Sellable: true, Purchasable: true, Active: true,
	}
	require.NoError(t, db.Create(&box).Error)
	seedTier(t, db, prodID, box.ID, "box", 12, 5, 100000)

	resp, err := svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: prodID}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Product.PriceTiers, 1)

	require.NoError(t, db.Model(&model.ProductUnit{}).Where("id = ?", box.ID).Update("active", false).Error)

	resp, err = svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: prodID}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Product.PriceTiers, "archived unit's tiers are hidden")
}

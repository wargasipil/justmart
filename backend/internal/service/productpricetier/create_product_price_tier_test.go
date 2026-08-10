package productpricetier_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestCreateProductPriceTier_HappyPath(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, _ := seedProductWithUnits(t, db, "PT-C1")

	tier := create(t, ctx, svc, prodID, baseID, 12, 8500)
	require.NotEmpty(t, tier.Id)
	require.Equal(t, prodID, tier.ProductId)
	require.Equal(t, baseID, tier.ProductUnitId)
	require.Equal(t, int32(12), tier.MinQty)
	require.Equal(t, int64(8500), tier.Price)
	require.Equal(t, "pcs", tier.UnitName, "unit snapshotted for display")
	require.Equal(t, int64(1), tier.UnitFactor)
	require.NotZero(t, tier.CreatedAt)
}

// A tier on a larger unit snapshots that unit's name and factor.
func TestCreateProductPriceTier_OnLargerUnit(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, _, boxID := seedProductWithUnits(t, db, "PT-C2")

	tier := create(t, ctx, svc, prodID, boxID, 5, 100000)
	require.Equal(t, "box", tier.UnitName)
	require.Equal(t, int64(12), tier.UnitFactor)
}

// min_qty 0 and 1 are rejected: a 0/1 tier is just the unit's sell_price, and
// sale_items.tier_min_qty = 0 is the "no grosir" flag, so allowing 0 would let
// one bad row silently suppress every auto discount for the product.
func TestCreateProductPriceTier_MinQtyBelowTwoRejected(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, _ := seedProductWithUnits(t, db, "PT-C3")

	for _, minQty := range []int32{0, 1, -5} {
		_, err := svc.CreateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.CreateProductPriceTierRequest{
			ProductId: prodID, ProductUnitId: baseID, MinQty: minQty, Price: 8500,
		}))
		requireToken(t, err, connect.CodeInvalidArgument, "product_price_tier.min_qty_invalid")
	}
}

func TestCreateProductPriceTier_NegativePriceRejected(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, _ := seedProductWithUnits(t, db, "PT-C4")

	_, err := svc.CreateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.CreateProductPriceTierRequest{
		ProductId: prodID, ProductUnitId: baseID, MinQty: 12, Price: -1,
	}))
	requireToken(t, err, connect.CodeInvalidArgument, "product_price_tier.price_invalid")
}

// A tier priced above the unit's sell_price is ACCEPTED — sell_price legitimately
// moves later, so the "is it cheaper?" question belongs to POS, not to storage.
func TestCreateProductPriceTier_AbovePriceAccepted(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, _ := seedProductWithUnits(t, db, "PT-C5")

	tier := create(t, ctx, svc, prodID, baseID, 12, 99000)
	require.Equal(t, int64(99000), tier.Price)
}

func TestCreateProductPriceTier_UnitRequired(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, _, _ := seedProductWithUnits(t, db, "PT-C6")

	_, err := svc.CreateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.CreateProductPriceTierRequest{
		ProductId: prodID, ProductUnitId: "", MinQty: 12, Price: 8500,
	}))
	requireToken(t, err, connect.CodeFailedPrecondition, "product_price_tier.unit_invalid")
}

// The unit must belong to THIS product — otherwise a tier could price another
// product's unit and corrupt the denormalized product_id.
func TestCreateProductPriceTier_UnitOfAnotherProductRejected(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodA, _, _ := seedProductWithUnits(t, db, "PT-C7a")
	_, otherBase, _ := seedProductWithUnits(t, db, "PT-C7b")

	_, err := svc.CreateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.CreateProductPriceTierRequest{
		ProductId: prodA, ProductUnitId: otherBase, MinQty: 12, Price: 8500,
	}))
	requireToken(t, err, connect.CodeFailedPrecondition, "product_price_tier.unit_invalid")
}

func TestCreateProductPriceTier_ProductMissing(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	_, baseID, _ := seedProductWithUnits(t, db, "PT-C8")

	_, err := svc.CreateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.CreateProductPriceTierRequest{
		ProductId: "", ProductUnitId: baseID, MinQty: 12, Price: 8500,
	}))
	requireToken(t, err, connect.CodeInvalidArgument, "product_price_tier.product_missing")

	_, err = svc.CreateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.CreateProductPriceTierRequest{
		ProductId: "11111111-1111-1111-1111-111111111111", ProductUnitId: baseID, MinQty: 12, Price: 8500,
	}))
	requireToken(t, err, connect.CodeFailedPrecondition, "product_price_tier.product_missing")
}

func TestCreateProductPriceTier_DuplicateRungRejected(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, boxID := seedProductWithUnits(t, db, "PT-C9")
	create(t, ctx, svc, prodID, baseID, 12, 8500)

	_, err := svc.CreateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.CreateProductPriceTierRequest{
		ProductId: prodID, ProductUnitId: baseID, MinQty: 12, Price: 8000,
	}))
	requireToken(t, err, connect.CodeAlreadyExists, "product_price_tier.tier_taken")

	// Same threshold on a DIFFERENT unit is a different rung and is fine.
	other := create(t, ctx, svc, prodID, boxID, 12, 100000)
	require.Equal(t, boxID, other.ProductUnitId)
}

package productpricetier_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func TestUpdateProductPriceTier_ChangesThresholdPriceAndUnit(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, boxID := seedProductWithUnits(t, db, "PT-U1")
	tier := create(t, ctx, svc, prodID, baseID, 12, 8500)

	resp, err := svc.UpdateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductPriceTierRequest{
		Id: tier.Id, ProductUnitId: boxID, MinQty: 5, Price: 100000,
	}))
	require.NoError(t, err)
	got := resp.Msg.Tier
	require.Equal(t, boxID, got.ProductUnitId)
	require.Equal(t, int32(5), got.MinQty)
	require.Equal(t, int64(100000), got.Price)
	require.Equal(t, "box", got.UnitName, "unit snapshot re-taken")
	require.Equal(t, int64(12), got.UnitFactor)
	require.Equal(t, prodID, got.ProductId, "product_id is immutable")
}

// Re-saving a tier without changing its (unit, min_qty) must not trip the
// uniqueness pre-check against itself.
func TestUpdateProductPriceTier_SelfExclusion(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, _ := seedProductWithUnits(t, db, "PT-U2")
	tier := create(t, ctx, svc, prodID, baseID, 12, 8500)

	resp, err := svc.UpdateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductPriceTierRequest{
		Id: tier.Id, ProductUnitId: baseID, MinQty: 12, Price: 8200,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(8200), resp.Msg.Tier.Price)
}

func TestUpdateProductPriceTier_OntoExistingRungRejected(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, _ := seedProductWithUnits(t, db, "PT-U3")
	create(t, ctx, svc, prodID, baseID, 12, 8500)
	second := create(t, ctx, svc, prodID, baseID, 60, 8000)

	_, err := svc.UpdateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductPriceTierRequest{
		Id: second.Id, ProductUnitId: baseID, MinQty: 12, Price: 8000,
	}))
	requireToken(t, err, connect.CodeAlreadyExists, "product_price_tier.tier_taken")
}

func TestUpdateProductPriceTier_Validation(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, _ := seedProductWithUnits(t, db, "PT-U4")
	tier := create(t, ctx, svc, prodID, baseID, 12, 8500)

	_, err := svc.UpdateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductPriceTierRequest{
		Id: tier.Id, ProductUnitId: baseID, MinQty: 1, Price: 8500,
	}))
	requireToken(t, err, connect.CodeInvalidArgument, "product_price_tier.min_qty_invalid")

	_, err = svc.UpdateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductPriceTierRequest{
		Id: tier.Id, ProductUnitId: baseID, MinQty: 12, Price: -1,
	}))
	requireToken(t, err, connect.CodeInvalidArgument, "product_price_tier.price_invalid")

	_, err = svc.UpdateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductPriceTierRequest{
		Id: "22222222-2222-2222-2222-222222222222", ProductUnitId: baseID, MinQty: 12, Price: 8500,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDeleteProductPriceTier(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, _ := seedProductWithUnits(t, db, "PT-D1")
	tier := create(t, ctx, svc, prodID, baseID, 12, 8500)

	_, err := svc.DeleteProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.DeleteProductPriceTierRequest{Id: tier.Id}))
	require.NoError(t, err)

	var n int64
	require.NoError(t, db.Model(&model.ProductPriceTier{}).Where("id = ?", tier.Id).Count(&n).Error)
	require.Zero(t, n, "hard delete, mirroring DeleteProductDiscount")

	// The rung is free again after deletion.
	create(t, ctx, svc, prodID, baseID, 12, 8000)
}

func TestDeleteProductPriceTier_NotFound(t *testing.T) {
	t.Parallel()
	svc, _, ctx := newSvc(t)

	_, err := svc.DeleteProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.DeleteProductPriceTierRequest{
		Id: "33333333-3333-3333-3333-333333333333",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

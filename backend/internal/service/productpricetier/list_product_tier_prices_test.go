package productpricetier_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	productpricetiersvc "github.com/justmart/backend/internal/service/productpricetier"
)

func history(t *testing.T, ctx context.Context, svc *productpricetiersvc.ProductPriceTierService, productID string) []*inventoryifacev1.ProductTierPrice {
	t.Helper()
	r, err := svc.ListProductTierPrices(ctx, connect.NewRequest(&inventoryifacev1.ListProductTierPricesRequest{
		ProductId: productID, Limit: 100,
	}))
	require.NoError(t, err)
	return r.Msg.Prices
}

// open returns the single open (still-current) row for a rung.
func open(t *testing.T, rows []*inventoryifacev1.ProductTierPrice, unitID string, minQty int32) *inventoryifacev1.ProductTierPrice {
	t.Helper()
	var found *inventoryifacev1.ProductTierPrice
	for _, r := range rows {
		if r.ProductUnitId == unitID && r.MinQty == minQty && r.EffectiveTo == 0 {
			require.Nil(t, found, "more than one open row for the same rung")
			found = r
		}
	}
	return found
}

func TestCreateProductPriceTier_OpensHistoryRow(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, _ := seedProductWithUnits(t, db, "PT-H1")
	create(t, ctx, svc, prodID, baseID, 12, 8500)

	rows := history(t, ctx, svc, prodID)
	require.Len(t, rows, 1)
	require.Equal(t, int64(8500), rows[0].Price)
	require.Equal(t, int32(12), rows[0].MinQty)
	require.Equal(t, "pcs", rows[0].UnitName)
	require.Zero(t, rows[0].EffectiveTo, "the new row is open")
	require.NotEmpty(t, rows[0].ChangedBy, "the caller is recorded")
	require.NotZero(t, rows[0].EffectiveFrom)
}

func TestUpdateProductPriceTier_ClosesAndOpensOnPriceChange(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, _ := seedProductWithUnits(t, db, "PT-H2")
	tier := create(t, ctx, svc, prodID, baseID, 12, 8500)

	_, err := svc.UpdateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductPriceTierRequest{
		Id: tier.Id, ProductUnitId: baseID, MinQty: 12, Price: 8000,
	}))
	require.NoError(t, err)

	rows := history(t, ctx, svc, prodID)
	require.Len(t, rows, 2, "the old price is kept, not overwritten")
	cur := open(t, rows, baseID, 12)
	require.NotNil(t, cur)
	require.Equal(t, int64(8000), cur.Price)

	var closed int
	for _, r := range rows {
		if r.EffectiveTo != 0 {
			closed++
			require.Equal(t, int64(8500), r.Price, "the superseded price survives")
		}
	}
	require.Equal(t, 1, closed)
}

// Re-saving with nothing priced differently must not append a duplicate row —
// otherwise every form submit inflates the history.
func TestUpdateProductPriceTier_NoOpWritesNoHistory(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, _ := seedProductWithUnits(t, db, "PT-H3")
	tier := create(t, ctx, svc, prodID, baseID, 12, 8500)

	_, err := svc.UpdateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductPriceTierRequest{
		Id: tier.Id, ProductUnitId: baseID, MinQty: 12, Price: 8500,
	}))
	require.NoError(t, err)
	require.Len(t, history(t, ctx, svc, prodID), 1)
}

// Moving a tier is two facts: the rung it left stops having a price, and the
// rung it arrived at starts having one.
func TestUpdateProductPriceTier_MovedRungClosesTheOldOne(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, boxID := seedProductWithUnits(t, db, "PT-H4")
	tier := create(t, ctx, svc, prodID, baseID, 12, 8500)

	_, err := svc.UpdateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductPriceTierRequest{
		Id: tier.Id, ProductUnitId: boxID, MinQty: 5, Price: 100000,
	}))
	require.NoError(t, err)

	rows := history(t, ctx, svc, prodID)
	require.Len(t, rows, 2)
	require.Nil(t, open(t, rows, baseID, 12), "the abandoned rung is closed")
	cur := open(t, rows, boxID, 5)
	require.NotNil(t, cur, "the new rung is open")
	require.Equal(t, int64(100000), cur.Price)
	require.Equal(t, "box", cur.UnitName, "unit snapshot re-taken")
}

// The history is keyed by the rung, not the tier row, so it survives the hard
// delete — and a later re-create continues the same rung's story.
func TestDeleteProductPriceTier_ClosesRungAndSurvivesRecreate(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, _ := seedProductWithUnits(t, db, "PT-H5")
	tier := create(t, ctx, svc, prodID, baseID, 12, 8500)

	_, err := svc.DeleteProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.DeleteProductPriceTierRequest{Id: tier.Id}))
	require.NoError(t, err)

	rows := history(t, ctx, svc, prodID)
	require.Len(t, rows, 1, "the deleted tier's price is still on record")
	require.NotZero(t, rows[0].EffectiveTo, "the rung is closed, not open")

	create(t, ctx, svc, prodID, baseID, 12, 7500)
	rows = history(t, ctx, svc, prodID)
	require.Len(t, rows, 2)
	cur := open(t, rows, baseID, 12)
	require.NotNil(t, cur)
	require.Equal(t, int64(7500), cur.Price)
}

func TestListProductTierPrices_PagesAndCounts(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodID, baseID, _ := seedProductWithUnits(t, db, "PT-H6")
	tier := create(t, ctx, svc, prodID, baseID, 12, 8500)
	for _, p := range []int64{8400, 8300, 8200} {
		_, err := svc.UpdateProductPriceTier(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductPriceTierRequest{
			Id: tier.Id, ProductUnitId: baseID, MinQty: 12, Price: p,
		}))
		require.NoError(t, err)
	}

	first, err := svc.ListProductTierPrices(ctx, connect.NewRequest(&inventoryifacev1.ListProductTierPricesRequest{
		ProductId: prodID, Limit: 2,
	}))
	require.NoError(t, err)
	require.Len(t, first.Msg.Prices, 2)
	require.Equal(t, int32(4), first.Msg.Total, "total ignores the page window")

	second, err := svc.ListProductTierPrices(ctx, connect.NewRequest(&inventoryifacev1.ListProductTierPricesRequest{
		ProductId: prodID, Limit: 2, Offset: 2,
	}))
	require.NoError(t, err)
	require.Len(t, second.Msg.Prices, 2)

	// No row may appear on both pages — the ORDER BY tiebreaker is what prevents it.
	seen := map[string]bool{}
	for _, r := range append(first.Msg.Prices, second.Msg.Prices...) {
		require.False(t, seen[r.Id], "row %s repeated across pages", r.Id)
		seen[r.Id] = true
	}
}

func TestListProductTierPrices_ScopedToProductAndValidated(t *testing.T) {
	t.Parallel()
	svc, db, ctx := newSvc(t)
	prodA, baseA, _ := seedProductWithUnits(t, db, "PT-H7a")
	prodB, baseB, _ := seedProductWithUnits(t, db, "PT-H7b")
	create(t, ctx, svc, prodA, baseA, 12, 8500)
	create(t, ctx, svc, prodB, baseB, 6, 9000)

	rows := history(t, ctx, svc, prodA)
	require.Len(t, rows, 1)
	require.Equal(t, prodA, rows[0].ProductId)

	_, err := svc.ListProductTierPrices(ctx, connect.NewRequest(&inventoryifacev1.ListProductTierPricesRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

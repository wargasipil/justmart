package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// seedTier adds one grosir (wholesale) rung to a product unit: buy >= minQty of
// that unit and the whole line re-prices to `price` per unit.
func seedTier(t *testing.T, db *gorm.DB, productID, unitID string, minQty int32, price int64) string {
	t.Helper()
	tier := model.ProductPriceTier{
		ProductID:     productID,
		ProductUnitID: unitID,
		UnitName:      "tab",
		UnitFactor:    1,
		MinQty:        minQty,
		Price:         price,
	}
	require.NoError(t, db.Create(&tier).Error)
	return tier.ID
}

// baseUnitID returns a product's base unit id (seedProduct creates exactly one).
func baseUnitID(t *testing.T, db *gorm.DB, productID string) string {
	t.Helper()
	var u model.ProductUnit
	require.NoError(t, db.Where("product_id = ? AND is_base", productID).First(&u).Error)
	return u.ID
}

// ladder is the plan's baseline fixture: 10.000 normally, 8.500 from 12, 8.000
// from 60, 7.200 from 144.
func ladder(t *testing.T, db *gorm.DB, sku string) (productID, unitID string) {
	t.Helper()
	productID = seedProduct(t, db, sku, "Kopi sachet", 10000)
	unitID = baseUnitID(t, db, productID)
	seedTier(t, db, productID, unitID, 12, 8500)
	seedTier(t, db, productID, unitID, 60, 8000)
	seedTier(t, db, productID, unitID, 144, 7200)
	return productID, unitID
}

func TestPriceTier_AppliedOnAdd_ReplacesUnitPrice(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID, _ := ladder(t, db, "GR1")
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 12}))
	require.NoError(t, err)
	it := add.Msg.Sale.Items[0]
	require.Equal(t, int64(8500), it.UnitPriceSnapshot, "tier replaces the unit price")
	require.Equal(t, int64(10000), it.ListPriceSnapshot, "normal price stays frozen for the UI + re-resolution")
	require.Equal(t, int32(12), it.TierMinQty)
	require.Equal(t, int64(0), it.LineDiscount)
	require.Equal(t, int64(102000), it.LineTotal)
	require.Equal(t, int64(102000), add.Msg.Sale.Total)
}

func TestPriceTier_BelowThreshold_NoTier(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID, _ := ladder(t, db, "GR2")
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 11}))
	require.NoError(t, err)
	it := add.Msg.Sale.Items[0]
	require.Equal(t, int64(10000), it.UnitPriceSnapshot)
	require.Equal(t, int32(0), it.TierMinQty)
	require.Equal(t, int64(110000), it.LineTotal)
}

func TestPriceTier_LadderPicksLowestQualifying(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID, _ := ladder(t, db, "GR3")

	for _, tc := range []struct {
		qty       int32
		wantPrice int64
		wantMin   int32
	}{
		{12, 8500, 12},
		{59, 8500, 12},
		{60, 8000, 60},
		{144, 7200, 144},
		{200, 7200, 144},
	} {
		saleID := startDraft(t, svc, ctx)
		add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: tc.qty}))
		require.NoError(t, err)
		it := add.Msg.Sale.Items[0]
		require.Equal(t, tc.wantPrice, it.UnitPriceSnapshot, "qty %d", tc.qty)
		require.Equal(t, tc.wantMin, it.TierMinQty, "qty %d", tc.qty)
	}
}

// A mis-entered ladder where a higher rung is DEARER stays harmless: the lowest
// qualifying price wins regardless of threshold order.
func TestPriceTier_NonMonotonicLadderPicksCheapest(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "GR4", "Gula", 10000)
	unitID := baseUnitID(t, db, prodID)
	seedTier(t, db, prodID, unitID, 12, 8000)
	seedTier(t, db, prodID, unitID, 60, 8500) // dearer than the rung below
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 60}))
	require.NoError(t, err)
	it := add.Msg.Sale.Items[0]
	require.Equal(t, int64(8000), it.UnitPriceSnapshot)
	require.Equal(t, int32(12), it.TierMinQty, "reports the rung the customer actually earned")
}

// The anti-ratchet: lowering the qty back below a threshold must restore the
// normal price. This is why list_price_snapshot exists.
func TestPriceTier_QtyDropReverts(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID, _ := ladder(t, db, "GR5")
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 60}))
	require.NoError(t, err)
	require.Equal(t, int64(8000), add.Msg.Sale.Items[0].UnitPriceSnapshot)
	itemID := add.Msg.Sale.Items[0].Id

	q, err := svc.SetItemQuantity(ctx, connect.NewRequest(&posifacev1.SetItemQuantityRequest{SaleId: saleID, ItemId: itemID, Qty: 5}))
	require.NoError(t, err)
	it := q.Msg.Sale.Items[0]
	require.Equal(t, int64(10000), it.UnitPriceSnapshot, "must revert, not ratchet")
	require.Equal(t, int32(0), it.TierMinQty)
	require.Equal(t, int64(10000), it.ListPriceSnapshot)
	require.Equal(t, int64(50000), it.LineTotal)
}

// Adding twice merges into one line, and the merged qty re-prices the WHOLE line
// (6 + 6 = 12 buys all 12 at the tier price).
func TestPriceTier_MergePathRePricesWholeLine(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID, _ := ladder(t, db, "GR6")
	saleID := startDraft(t, svc, ctx)

	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 6}))
	require.NoError(t, err)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 6}))
	require.NoError(t, err)

	require.Len(t, add.Msg.Sale.Items, 1)
	it := add.Msg.Sale.Items[0]
	require.Equal(t, int32(12), it.Qty)
	require.Equal(t, int64(8500), it.UnitPriceSnapshot)
	require.Equal(t, int64(102000), it.LineTotal)
}

// Grosir wins: the automatic product discount is suppressed AND cleared, so a
// discount earned at a lower qty can't survive the bump that crossed the tier.
func TestPriceTier_SuppressesAutoProductDiscount(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID, _ := ladder(t, db, "GR7")
	seedProductDiscount(t, db, prodID, "PERCENT", false, 1000, 0, nil) // 10%, always
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 11}))
	require.NoError(t, err)
	it := add.Msg.Sale.Items[0]
	require.Equal(t, int64(10000), it.UnitPriceSnapshot)
	require.Equal(t, int64(11000), it.LineDiscount, "auto discount applies below the tier")
	itemID := it.Id

	q, err := svc.SetItemQuantity(ctx, connect.NewRequest(&posifacev1.SetItemQuantityRequest{SaleId: saleID, ItemId: itemID, Qty: 12}))
	require.NoError(t, err)
	it = q.Msg.Sale.Items[0]
	require.Equal(t, int64(8500), it.UnitPriceSnapshot)
	require.Equal(t, int64(0), it.LineDiscount, "auto discount suppressed under grosir")
	require.Equal(t, int64(0), it.DiscountValue, "and CLEARED, not merely skipped")
	require.Equal(t, int64(102000), it.LineTotal)
}

func TestPriceTier_ManualLineDiscountStillApplies(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID, _ := ladder(t, db, "GR8")
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 12}))
	require.NoError(t, err)
	itemID := add.Msg.Sale.Items[0].Id

	// 10% off the GROSIR gross of 102.000.
	d, err := svc.SetLineDiscount(ctx, connect.NewRequest(&posifacev1.SetLineDiscountRequest{
		SaleId: saleID, ItemId: itemID, DiscountType: "PERCENT", DiscountValue: 1000,
	}))
	require.NoError(t, err)
	it := d.Msg.Sale.Items[0]
	require.Equal(t, int64(8500), it.UnitPriceSnapshot)
	require.True(t, it.DiscountManual)
	require.Equal(t, int64(10200), it.LineDiscount)
	require.Equal(t, int64(91800), it.LineTotal)

	// Clearing it returns to plain grosir — the auto discount stays suppressed.
	c, err := svc.ClearLineDiscount(ctx, connect.NewRequest(&posifacev1.ClearLineDiscountRequest{SaleId: saleID, ItemId: itemID}))
	require.NoError(t, err)
	it = c.Msg.Sale.Items[0]
	require.Equal(t, int64(8500), it.UnitPriceSnapshot)
	require.Equal(t, int64(0), it.LineDiscount)
	require.Equal(t, int64(102000), it.LineTotal)
}

// A tier priced at or above the normal price gives no benefit, so it must NOT
// apply — otherwise it would set the grosir flag and suppress the auto discount
// for nothing, leaving the customer strictly worse off than with no tier at all.
func TestPriceTier_NeverExceedsListPrice(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "GR9", "Teh", 10000)
	unitID := baseUnitID(t, db, prodID)
	seedTier(t, db, prodID, unitID, 2, 12000)                          // dearer than normal
	seedProductDiscount(t, db, prodID, "PERCENT", false, 1000, 0, nil) // 10%
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 5}))
	require.NoError(t, err)
	it := add.Msg.Sale.Items[0]
	require.Equal(t, int64(10000), it.UnitPriceSnapshot)
	require.Equal(t, int32(0), it.TierMinQty)
	require.Equal(t, int64(5000), it.LineDiscount, "auto discount must survive a useless tier")
}

// Equal-to-list is also no benefit (the apply condition is strict).
func TestPriceTier_EqualToListPriceNotApplied(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "GR10", "Susu", 10000)
	seedTier(t, db, prodID, baseUnitID(t, db, prodID), 2, 10000)
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 5}))
	require.NoError(t, err)
	require.Equal(t, int32(0), add.Msg.Sale.Items[0].TierMinQty)
}

func TestPriceTier_TierPriceZero(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "GR11", "Sampel", 10000)
	seedTier(t, db, prodID, baseUnitID(t, db, prodID), 2, 0)
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 2}))
	require.NoError(t, err)
	require.Equal(t, int64(0), add.Msg.Sale.Items[0].LineTotal)
	require.Equal(t, int64(0), add.Msg.Sale.Total)
}

// Tiers bind to their OWN unit: a pcs ladder is not earned by buying boxes, and
// the two lines coexist without cross-unit aggregation.
func TestPriceTier_OtherUnitLineUnaffected(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID, _ := ladder(t, db, "GR12")
	box := model.ProductUnit{ProductID: prodID, Name: "box", Factor: 12, SellPrice: 110000, Sellable: true, Active: true}
	require.NoError(t, db.Create(&box).Error)
	saleID := startDraft(t, svc, ctx)

	// 12 boxes = 144 base units — well past the pcs ladder's top rung, but the
	// box line has no ladder of its own.
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, ProductUnitId: box.ID, Qty: 12,
	}))
	require.NoError(t, err)
	boxLine := add.Msg.Sale.Items[0]
	require.Equal(t, int64(110000), boxLine.UnitPriceSnapshot)
	require.Equal(t, int32(0), boxLine.TierMinQty)

	// A pcs line on the same cart resolves its own ladder independently.
	add2, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 12}))
	require.NoError(t, err)
	require.Len(t, add2.Msg.Sale.Items, 2)
	var pcsLine *posifacev1.SaleItem
	for _, it := range add2.Msg.Sale.Items {
		if it.UnitName == "tab" {
			pcsLine = it
		}
	}
	require.NotNil(t, pcsLine)
	require.Equal(t, int64(8500), pcsLine.UnitPriceSnapshot)
}

// Deleting a tier mid-draft leaves the line priced until its next qty change,
// then it reverts — same as a deleted product discount or an edited sell_price.
func TestPriceTier_TierDeletedMidDraft_RevertsOnNextQtyChange(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "GR13", "Beras", 10000)
	tierID := seedTier(t, db, prodID, baseUnitID(t, db, prodID), 12, 8500)
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 12}))
	require.NoError(t, err)
	require.Equal(t, int64(8500), add.Msg.Sale.Items[0].UnitPriceSnapshot)
	itemID := add.Msg.Sale.Items[0].Id

	require.NoError(t, db.Where("id = ?", tierID).Delete(&model.ProductPriceTier{}).Error)

	q, err := svc.SetItemQuantity(ctx, connect.NewRequest(&posifacev1.SetItemQuantityRequest{SaleId: saleID, ItemId: itemID, Qty: 13}))
	require.NoError(t, err)
	require.Equal(t, int64(10000), q.Msg.Sale.Items[0].UnitPriceSnapshot)
	require.Equal(t, int32(0), q.Msg.Sale.Items[0].TierMinQty)
}

// A pre-UOM line (NULL product_unit_id, no list price) must not panic and must
// keep its price — applyTierPrice no-ops on it.
func TestPriceTier_LegacyLineWithoutUnit(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "GR14", "Legacy", 10000)
	saleID := startDraft(t, svc, ctx)

	legacy := model.SaleItem{
		SaleID:            saleID,
		ProductID:         prodID,
		ProductUnitID:     nil,
		UnitFactor:        1,
		Qty:               3,
		BaseQty:           3,
		UnitPriceSnapshot: 9000,
		ListPriceSnapshot: 0, // predates the column
		LineTotal:         27000,
	}
	require.NoError(t, db.Create(&legacy).Error)

	q, err := svc.SetItemQuantity(ctx, connect.NewRequest(&posifacev1.SetItemQuantityRequest{SaleId: saleID, ItemId: legacy.ID, Qty: 20}))
	require.NoError(t, err)
	it := q.Msg.Sale.Items[0]
	require.Equal(t, int64(9000), it.UnitPriceSnapshot, "legacy price preserved")
	require.Equal(t, int32(0), it.TierMinQty)
	require.Equal(t, int64(180000), it.LineTotal)
}

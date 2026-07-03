package sale_test

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func seedProductDiscount(t *testing.T, db *gorm.DB, productID, typ string, perItem bool, value int64, minQty int32, expires *time.Time) {
	t.Helper()
	require.NoError(t, db.Create(&model.ProductDiscount{
		ProductID: productID, DiscountType: typ, PerItem: perItem, Value: value, MinQty: minQty, MinQtyUnitFactor: 1, ExpiresAt: expires,
	}).Error)
}

// seedProductDiscountUnit seeds a discount whose min-qty threshold is in a
// specific unit (unit-aware rule: base_qty >= minQty × unitFactor).
func seedProductDiscountUnit(t *testing.T, db *gorm.DB, productID, typ string, perItem bool, value int64, minQty int32, unitID, unitName string, unitFactor int64) {
	t.Helper()
	require.NoError(t, db.Create(&model.ProductDiscount{
		ProductID: productID, DiscountType: typ, PerItem: perItem, Value: value,
		MinQty: minQty, MinQtyUnitID: &unitID, MinQtyUnitName: unitName, MinQtyUnitFactor: unitFactor,
	}).Error)
}

// The min-qty rule is unit-aware and compares in BASE units: "min 1 box (×12)"
// requires 12 base units regardless of the selling unit used.
func TestProductDiscount_MinQtyUnitComparesBaseUnits(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "PDU1", "Kopi", 1000)
	box := model.ProductUnit{ProductID: prodID, Name: "box", Factor: 12, SellPrice: 11000, Sellable: true, Active: true}
	require.NoError(t, db.Create(&box).Error)
	seedProductDiscountUnit(t, db, prodID, "PERCENT", false, 1000, 1, box.ID, "box", 12) // 10% when buying ≥ 1 box (=12 base)
	saleID := startDraft(t, svc, ctx)

	// 11 base units (tab) — below the 12-base threshold → no discount.
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 11}))
	require.NoError(t, err)
	require.Equal(t, int64(0), add.Msg.Sale.Items[0].LineDiscount)
	itemID := add.Msg.Sale.Items[0].Id

	// 12 base units → threshold met → 10% of 12,000 = 1,200.
	q, err := svc.SetItemQuantity(ctx, connect.NewRequest(&posifacev1.SetItemQuantityRequest{SaleId: saleID, ItemId: itemID, Qty: 12}))
	require.NoError(t, err)
	require.Equal(t, int64(1200), q.Msg.Sale.Items[0].LineDiscount)

	// Selling 1 box directly (12 base units at box price 11,000) also qualifies:
	// 10% of 11,000 = 1,100 on the box line.
	sale2 := startDraft(t, svc, ctx)
	add2, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: sale2, ProductId: prodID, ProductUnitId: box.ID, Qty: 1}))
	require.NoError(t, err)
	require.Equal(t, int64(1100), add2.Msg.Sale.Items[0].LineDiscount)
}

func TestProductDiscount_AutoAppliedOnAdd(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "PD1", "Antasida", 1000)
	seedProductDiscount(t, db, prodID, "PERCENT", false, 1000, 0, nil) // 10%
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 2}))
	require.NoError(t, err)
	it := add.Msg.Sale.Items[0]
	require.Equal(t, int64(200), it.LineDiscount) // 10% of 2000
	require.Equal(t, int64(1800), it.LineTotal)
	require.False(t, it.DiscountManual) // auto
	require.Equal(t, int64(1800), add.Msg.Sale.Total)
}

func TestProductDiscount_MinQtyGate(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "PD2", "Paracetamol", 1000)
	seedProductDiscount(t, db, prodID, "PERCENT", false, 1000, 3, nil) // 10%, buy ≥3
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 2}))
	require.NoError(t, err)
	require.Equal(t, int64(0), add.Msg.Sale.Items[0].LineDiscount) // below threshold

	itemID := add.Msg.Sale.Items[0].Id
	q, err := svc.SetItemQuantity(ctx, connect.NewRequest(&posifacev1.SetItemQuantityRequest{SaleId: saleID, ItemId: itemID, Qty: 3}))
	require.NoError(t, err)
	require.Equal(t, int64(300), q.Msg.Sale.Items[0].LineDiscount) // 10% of 3000, threshold met
}

func TestProductDiscount_PerItemFixed(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "PD3", "Vitamin", 1000)
	seedProductDiscount(t, db, prodID, "FIXED", true, 500, 0, nil) // Rp500 off per item
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 3}))
	require.NoError(t, err)
	it := add.Msg.Sale.Items[0]
	require.Equal(t, int64(1500), it.LineDiscount) // 500 × 3
	require.True(t, it.DiscountPerItem)
	require.Equal(t, int64(1500), it.LineTotal)
}

func TestProductDiscount_BestOfSeveralWins(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "PD4", "Obat", 1000)
	seedProductDiscount(t, db, prodID, "PERCENT", false, 1000, 0, nil) // 10% → 200 on qty2
	seedProductDiscount(t, db, prodID, "FIXED", true, 150, 0, nil)     // 150×2 = 300 on qty2
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 2}))
	require.NoError(t, err)
	it := add.Msg.Sale.Items[0]
	require.Equal(t, int64(300), it.LineDiscount) // the bigger one wins
	require.Equal(t, "FIXED", it.DiscountType)
	require.True(t, it.DiscountPerItem)
}

func TestProductDiscount_ExpiredIgnored(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	prodID := seedProduct(t, db, "PD5", "Salep", 1000)
	yesterday := time.Now().AddDate(0, 0, -1)
	seedProductDiscount(t, db, prodID, "PERCENT", false, 1000, 0, &yesterday) // expired
	saleID := startDraft(t, svc, ctx)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 2}))
	require.NoError(t, err)
	require.Equal(t, int64(0), add.Msg.Sale.Items[0].LineDiscount) // expired → no discount
}

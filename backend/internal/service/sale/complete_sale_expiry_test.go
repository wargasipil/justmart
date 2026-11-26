package sale_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
	salesvc "github.com/justmart/backend/internal/service/sale"
)

// These pin the pharmacy expiry rule at checkout: CompleteSale's FEFO never
// takes from a lot whose expiry date is BEFORE today (shop-local day), a lot is
// still sellable ON its expiry date, and a line that cannot be covered without
// expired stock refuses the whole sale with sale.stock_expired. Retail keeps
// selling FEFO exactly as before.

// Expiry dates mean "relative to today", so they are COMPUTED (CLAUDE.md HARD
// RULE), with the same names the prescription tests and frontend fixtures use.
func daysAgo(n int) string { return time.Now().AddDate(0, 0, -n).Format(common.DateLayout) }
func dateIn(n int) string  { return time.Now().AddDate(0, 0, n).Format(common.DateLayout) }

// defaultWarehouseID resolves the migration-seeded default warehouse at runtime
// (the one an OWNER with no membership sells from).
func defaultWarehouseID(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var wh model.Warehouse
	require.NoError(t, db.Where("is_default").First(&wh).Error)
	return wh.ID
}

// seedLot creates a lot of productID expiring on day (YYYY-MM-DD) with qty base
// units on hand in the default warehouse. The date is stored the way the app
// stores one (parsed YYYY-MM-DD, a zone-free calendar day).
func seedLot(t *testing.T, db *gorm.DB, productID, userID, batchNo, day string, qty int32) string {
	t.Helper()
	expiry, err := time.Parse(common.DateLayout, day)
	require.NoError(t, err)
	b := model.Batch{
		ProductID:   productID,
		BatchNumber: batchNo,
		ExpiryDate:  expiry,
		CostPrice:   500,
		ReceivedAt:  time.Now(),
	}
	require.NoError(t, db.Create(&b).Error)
	require.NoError(t, db.Create(&model.StockMovement{
		BatchID:     b.ID,
		Qty:         qty,
		Type:        "PURCHASE",
		Reason:      "opening",
		UserID:      userID,
		WarehouseID: defaultWarehouseID(t, db),
	}).Error)
	return b.ID
}

// draftWith opens a DRAFT and adds qty base units of productID to it.
func draftWith(t *testing.T, svc *salesvc.SaleService, ctx context.Context, productID string, qty int32) string {
	t.Helper()
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: qty,
	}))
	require.NoError(t, err)
	return saleID
}

func checkout(svc *salesvc.SaleService, ctx context.Context, saleID string) (*connect.Response[posifacev1.CompleteSaleResponse], error) {
	return svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        saleID,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    1_000_000,
	}))
}

// saleMovements returns the sale's SALE movements as batch id -> qty (negative).
func saleMovements(t *testing.T, db *gorm.DB, saleID string) map[string]int32 {
	t.Helper()
	var mvs []model.StockMovement
	require.NoError(t, db.
		Where("type = ? AND sale_item_id IN (?)", "SALE",
			db.Model(&model.SaleItem{}).Select("id").Where("sale_id = ?", saleID)).
		Find(&mvs).Error)
	out := make(map[string]int32, len(mvs))
	for _, m := range mvs {
		out[m.BatchID] += m.Qty
	}
	return out
}

// requireRefused asserts a FailedPrecondition carrying token, that no SALE
// movement was written, and that the sale is still an open DRAFT.
func requireRefused(t *testing.T, svc *salesvc.SaleService, ctx context.Context, db *gorm.DB, saleID string, err error, token string) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	require.Equal(t, token, ce.Message())

	require.Empty(t, saleMovements(t, db, saleID), "a refused sale writes no stock movement")
	got, gerr := svc.GetSale(ctx, connect.NewRequest(&posifacev1.GetSaleRequest{Id: saleID}))
	require.NoError(t, gerr)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_DRAFT, got.Msg.Sale.Status)
}

func onHand(t *testing.T, db *gorm.DB, batchID string) int64 {
	t.Helper()
	n, err := common.BatchQtyInWarehouse(context.Background(), db, batchID, defaultWarehouseID(t, db))
	require.NoError(t, err)
	return n
}

// Pharmacy: the expired lot sorts first by FEFO but is skipped; the line is
// taken from the valid lot only.
func TestCompleteSale_Pharmacy_SkipsExpiredLot(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedProduct(t, db, "EXP-1", "Paracetamol", 1000)
	expiredLot := seedLot(t, db, prodID, ownerID, "OLD", daysAgo(1), 10)
	validLot := seedLot(t, db, prodID, ownerID, "NEW", dateIn(30), 10)
	saleID := draftWith(t, svc, ctx, prodID, 4)

	res, err := checkout(svc, ctx, saleID)
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_COMPLETED, res.Msg.Sale.Status)
	require.Equal(t, map[string]int32{validLot: -4}, saleMovements(t, db, saleID))
	require.Equal(t, int64(10), onHand(t, db, expiredLot), "the expired lot is untouched")
	require.Equal(t, int64(6), onHand(t, db, validLot))
}

// Pharmacy: only expired stock on hand -> sale.stock_expired, nothing written,
// the sale stays DRAFT so the cashier can fix the cart.
func TestCompleteSale_Pharmacy_OnlyExpiredStockRefused(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedProduct(t, db, "EXP-2", "Amoxicillin", 1000)
	lot := seedLot(t, db, prodID, ownerID, "OLD", daysAgo(1), 10)
	saleID := draftWith(t, svc, ctx, prodID, 2)

	_, err := checkout(svc, ctx, saleID)
	requireRefused(t, svc, ctx, db, saleID, err, "sale.stock_expired")
	require.Equal(t, int64(10), onHand(t, db, lot))
}

// Pharmacy: a lot expiring TODAY is still sellable.
func TestCompleteSale_Pharmacy_LotExpiringTodayIsSold(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedProduct(t, db, "EXP-3", "Ibuprofen", 1000)
	lot := seedLot(t, db, prodID, ownerID, "TODAY", daysAgo(0), 5)
	saleID := draftWith(t, svc, ctx, prodID, 5)

	res, err := checkout(svc, ctx, saleID)
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_COMPLETED, res.Msg.Sale.Status)
	require.Equal(t, map[string]int32{lot: -5}, saleMovements(t, db, saleID))
}

// Pharmacy: expired 5 + valid 3 cannot cover a line of 5 without the expired
// lot -> sale.stock_expired, and the 3 already taken from the valid lot during
// the walk are rolled back with the rest of the tx.
func TestCompleteSale_Pharmacy_MixedStockShortRefused(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedProduct(t, db, "EXP-4", "Cetirizine", 1000)
	expiredLot := seedLot(t, db, prodID, ownerID, "OLD", daysAgo(3), 5)
	validLot := seedLot(t, db, prodID, ownerID, "NEW", dateIn(60), 3)
	saleID := draftWith(t, svc, ctx, prodID, 5)

	_, err := checkout(svc, ctx, saleID)
	requireRefused(t, svc, ctx, db, saleID, err, "sale.stock_expired")
	require.Equal(t, int64(5), onHand(t, db, expiredLot))
	require.Equal(t, int64(3), onHand(t, db, validLot))
}

// Pharmacy: a lot recorded as "does not expire" (common.NoExpiryDate) is sold.
func TestCompleteSale_Pharmacy_NoExpiryLotIsSold(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedProduct(t, db, "EXP-5", "Kasa steril", 1000)
	lot := seedLot(t, db, prodID, ownerID, "NONE", common.NoExpiryDate.Format(common.DateLayout), 5)
	saleID := draftWith(t, svc, ctx, prodID, 2)

	_, err := checkout(svc, ctx, saleID)
	require.NoError(t, err)
	require.Equal(t, map[string]int32{lot: -2}, saleMovements(t, db, saleID))
}

// Retail: unchanged — FEFO still sells the expired lot first.
func TestCompleteSale_Retail_StillSellsExpiredLotFirst(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setRetailMode(t, db)
	prodID := seedProduct(t, db, "EXP-6", "Sabun", 1000)
	expiredLot := seedLot(t, db, prodID, ownerID, "OLD", daysAgo(1), 10)
	seedLot(t, db, prodID, ownerID, "NEW", dateIn(30), 10)
	saleID := draftWith(t, svc, ctx, prodID, 4)

	res, err := checkout(svc, ctx, saleID)
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_COMPLETED, res.Msg.Sale.Status)
	require.Equal(t, map[string]int32{expiredLot: -4}, saleMovements(t, db, saleID))
}

// No stock at all -> sale.insufficient_stock in both modes (the token replaced
// a plain-English message; the code is unchanged).
func TestCompleteSale_NoStock_InsufficientStockToken(t *testing.T) {
	t.Parallel()
	for name, setMode := range map[string]func(*testing.T, *gorm.DB){
		"retail":   setRetailMode,
		"pharmacy": setPharmacyMode,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, ctx, db, _ := newSaleSvc(t)
			setMode(t, db)
			prodID := seedProduct(t, db, "EXP-7", "Vitamin C", 1000)
			saleID := draftWith(t, svc, ctx, prodID, 1)

			_, err := checkout(svc, ctx, saleID)
			requireRefused(t, svc, ctx, db, saleID, err, "sale.insufficient_stock")
		})
	}
}

// Short on valid stock with no expired lot involved is plain insufficient
// stock in pharmacy mode too, not an expiry refusal.
func TestCompleteSale_Pharmacy_ShortWithoutExpiredIsInsufficient(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedProduct(t, db, "EXP-8", "Antasida", 1000)
	seedLot(t, db, prodID, ownerID, "NEW", dateIn(30), 1)
	saleID := draftWith(t, svc, ctx, prodID, 3)

	_, err := checkout(svc, ctx, saleID)
	requireRefused(t, svc, ctx, db, saleID, err, "sale.insufficient_stock")
}

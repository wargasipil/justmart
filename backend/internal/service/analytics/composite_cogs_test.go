package analytics_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	analyticsifacev1 "github.com/justmart/backend/gen/analytics_iface/v1"
	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
	analyticssvc "github.com/justmart/backend/internal/service/analytics"
	"github.com/justmart/backend/internal/service/common"
	salesvc "github.com/justmart/backend/internal/service/sale"
	"github.com/justmart/backend/internal/service/servicetest"
)

// This is the test behind the claim that recipes cost analytics NOTHING.
//
// COGS is computed from SALE stock_movements joined by sale_item_id, never from
// the sold product's own batches. A composite sale writes its INGREDIENT
// movements carrying the MENU LINE's sale_item_id, so the existing aggregation
// attributes their cost to the menu item with no analytics change at all. If
// that ever regresses — say someone "simplifies" the explosion to stamp the
// component's own id — a menu item's profit silently becomes 100% margin and its
// ingredients start showing phantom cost. This pins it.
//
// It deliberately goes through the REAL CompleteSale rather than hand-seeding
// movements: hand-seeding the very rows under test would beg the question.
func TestProductMetric_CompositeCostLandsOnTheMenuItem(t *testing.T) {
	t.Parallel()
	db, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, db, cfg)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	sales := salesvc.NewSaleService(db, cfg.Printer)
	svc := analyticssvc.NewAnalyticsService(db)

	var wh model.Warehouse
	require.NoError(t, db.Where("is_default").First(&wh).Error)

	// Ingredients, with a known cost per BASE unit.
	rice := seedStockedWithBatch(t, db, ownerID, wh.ID, "ing-rice", "Beras", "g", 100, 1000)
	egg := seedStockedWithBatch(t, db, ownerID, wh.ID, "ing-egg", "Telur", "butir", 2000, 10)

	// The menu item: no batches of its own, priced at 25 000.
	nasgor := seedProductWithUnit(t, db, "menu-nasgor", "Nasi Goreng", "porsi", 25000,
		common.ProductKindComposite)
	seedRecipe(t, db, nasgor, rice, 200) // 200 g per portion
	seedRecipe(t, db, nasgor, egg, 1)    // 1 egg per portion

	// Sell 2 portions through the real till.
	start, err := sales.StartSale(ctx, connect.NewRequest(&posifacev1.StartSaleRequest{}))
	require.NoError(t, err)
	saleID := start.Msg.Sale.Id
	_, err = sales.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: nasgor, Qty: 2,
	}))
	require.NoError(t, err)
	_, err = sales.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        saleID,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    50000,
	}))
	require.NoError(t, err)

	now := time.Now()
	resp, err := svc.ProductMetric(ctx, connect.NewRequest(&analyticsifacev1.ProductMetricRequest{
		MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
		Filter: &analyticsifacev1.Filter{
			FromUnix: now.AddDate(0, 0, -1).Unix(),
			ToUnix:   now.AddDate(0, 0, 1).Unix(),
		},
		Limit: 25,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Order)

	menu := resp.Msg.Order.Data[nasgor]
	require.NotNil(t, menu, "the menu item must appear in product analytics")
	require.Equal(t, int64(50000), menu.Terjual, "revenue = 2 x 25 000")
	// 2 portions x (200 g x Rp100 + 1 egg x Rp2 000) = 2 x 22 000.
	require.Equal(t, int64(44000), menu.Hpp, "food cost must be the exploded ingredient cost")
	require.Equal(t, int64(6000), menu.Profit)

	// The ingredients were CONSUMED, not sold. They still appear in the catalog
	// universe (ProductMetric emits a zero row for every product on the page),
	// but must carry NO revenue and NO cost of their own — otherwise the same
	// rupiah of food cost would be counted twice, once under the dish and again
	// under the ingredient, and margin would be nonsense on both.
	for _, ing := range []struct {
		id   string
		name string
	}{{rice, "rice"}, {egg, "egg"}} {
		if o := resp.Msg.Order.Data[ing.id]; o != nil {
			require.Zero(t, o.Terjual, "%s was consumed, not sold", ing.name)
			require.Zero(t, o.Hpp, "%s cost belongs to the dish, not to itself", ing.name)
		}
	}
}

// A SERVICE line has revenue and no cost — it consumes nothing, so 100% margin
// is the correct answer here rather than a missing-data artifact.
func TestProductMetric_ServiceLineHasRevenueAndNoCost(t *testing.T) {
	t.Parallel()
	db, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, db, cfg)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	sales := salesvc.NewSaleService(db, cfg.Printer)
	svc := analyticssvc.NewAnalyticsService(db)

	fee := seedProductWithUnit(t, db, "svc-corkage", "Corkage", "x", 15000, common.ProductKindService)

	start, err := sales.StartSale(ctx, connect.NewRequest(&posifacev1.StartSaleRequest{}))
	require.NoError(t, err)
	_, err = sales.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: start.Msg.Sale.Id, ProductId: fee, Qty: 1,
	}))
	require.NoError(t, err)
	_, err = sales.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        start.Msg.Sale.Id,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    15000,
	}))
	require.NoError(t, err)

	now := time.Now()
	resp, err := svc.ProductMetric(ctx, connect.NewRequest(&analyticsifacev1.ProductMetricRequest{
		MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
		Filter: &analyticsifacev1.Filter{
			FromUnix: now.AddDate(0, 0, -1).Unix(),
			ToUnix:   now.AddDate(0, 0, 1).Unix(),
		},
		Limit: 25,
	}))
	require.NoError(t, err)
	o := resp.Msg.Order.Data[fee]
	require.NotNil(t, o)
	require.Equal(t, int64(15000), o.Terjual)
	require.Equal(t, int64(0), o.Hpp)
	require.Equal(t, int64(15000), o.Profit)
}

// --- fixtures ---------------------------------------------------------------

// seedProductWithUnit inserts a product of an explicit kind plus its base unit
// (every sellable product needs one — a menu item is still priced in a unit).
func seedProductWithUnit(
	t *testing.T,
	db *gorm.DB,
	sku, name, unit string,
	price int64,
	kind string,
) string {
	t.Helper()
	p := model.Product{SKU: sku, Name: name, Unit: unit, UnitPrice: price, Active: true, Kind: kind}
	require.NoError(t, db.Create(&p).Error)
	require.NoError(t, db.Create(&model.ProductUnit{
		ProductID: p.ID, Name: unit, Factor: 1, IsBase: true, SellPrice: price, Sellable: true,
	}).Error)
	return p.ID
}

// seedStockedWithBatch inserts a STOCKED ingredient with one batch at a known
// cost per base unit and an opening PURCHASE movement.
func seedStockedWithBatch(
	t *testing.T,
	db *gorm.DB,
	userID, warehouseID string,
	sku, name, unit string,
	costPrice int64,
	qty int32,
) string {
	t.Helper()
	id := seedProductWithUnit(t, db, sku, name, unit, 0, common.ProductKindStocked)
	b := model.Batch{
		ProductID: id, BatchNumber: "B-" + sku, CostPrice: costPrice,
		ExpiryDate: time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC), ReceivedAt: time.Now(),
	}
	require.NoError(t, db.Create(&b).Error)
	require.NoError(t, db.Create(&model.StockMovement{
		BatchID: b.ID, Qty: qty, Type: "PURCHASE", Reason: "opening",
		UserID: userID, WarehouseID: warehouseID,
	}).Error)
	return id
}

func seedRecipe(t *testing.T, db *gorm.DB, parentID, componentID string, qtyBase int64) {
	t.Helper()
	require.NoError(t, db.Create(&model.ProductRecipeItem{
		ProductID: parentID, ComponentProductID: componentID, QtyBase: qtyBase,
	}).Error)
}

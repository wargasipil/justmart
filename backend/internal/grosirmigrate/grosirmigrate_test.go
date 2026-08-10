package grosirmigrate_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/grosirmigrate"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/servicetest"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	return servicetest.NewDB(t, servicetest.NewConfig(t))
}

// seedProduct inserts a product with a base "pcs" unit (10.000) and a "box" ×12
// unit (110.000), returning (productID, baseUnitID, boxUnitID).
func seedProduct(t *testing.T, db *gorm.DB, sku string) (string, string, string) {
	t.Helper()
	p := model.Product{SKU: sku, Name: sku + " Item", Unit: "pcs", UnitPrice: 10000, Active: true}
	require.NoError(t, db.Create(&p).Error)
	base := model.ProductUnit{
		ProductID: p.ID, Name: "pcs", Factor: 1, IsBase: true,
		SellPrice: 10000, Sellable: true, Purchasable: true, Active: true,
	}
	require.NoError(t, db.Create(&base).Error)
	box := model.ProductUnit{
		ProductID: p.ID, Name: "box", Factor: 12,
		SellPrice: 110000, Sellable: true, Purchasable: true, Active: true,
	}
	require.NoError(t, db.Create(&box).Error)
	return p.ID, base.ID, box.ID
}

func seedDiscount(t *testing.T, db *gorm.DB, d model.ProductDiscount) string {
	t.Helper()
	if d.MinQtyUnitFactor == 0 {
		d.MinQtyUnitFactor = 1
	}
	require.NoError(t, db.Create(&d).Error)
	return d.ID
}

func plan(t *testing.T, db *gorm.DB, opts grosirmigrate.Options) *grosirmigrate.Plan {
	t.Helper()
	p, err := grosirmigrate.BuildPlan(db, opts)
	require.NoError(t, err)
	return p
}

func TestBuildPlan_PercentDiscountBecomesTier(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	pid, baseID, _ := seedProduct(t, db, "SKU-PCT")
	seedDiscount(t, db, model.ProductDiscount{
		ProductID: pid, DiscountType: "PERCENT", Value: 1000, MinQty: 12, // 10% off from 12 pcs
	})

	got := plan(t, db, grosirmigrate.Options{})
	conv := got.Convertible()
	require.Len(t, conv, 1)
	require.Equal(t, baseID, conv[0].UnitID) // NULL threshold unit => base unit
	require.Equal(t, int32(12), conv[0].MinQty)
	require.Equal(t, int64(10000), conv[0].ListPrice)
	require.Equal(t, int64(9000), conv[0].TierPrice)
	require.Empty(t, got.Skipped())
}

func TestBuildPlan_ThresholdUnitDrivesTheTier(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	pid, _, boxID := seedProduct(t, db, "SKU-BOX")
	seedDiscount(t, db, model.ProductDiscount{
		ProductID: pid, DiscountType: "PERCENT", Value: 500, MinQty: 3,
		MinQtyUnitID: &boxID, MinQtyUnitName: "box", MinQtyUnitFactor: 12,
	})

	conv := plan(t, db, grosirmigrate.Options{}).Convertible()
	require.Len(t, conv, 1)
	require.Equal(t, boxID, conv[0].UnitID)
	require.Equal(t, "box", conv[0].UnitName)
	require.Equal(t, int64(12), conv[0].UnitFactor)
	require.Equal(t, int64(104500), conv[0].TierPrice) // 110.000 − 5%
}

func TestBuildPlan_PerItemFixedConvertsAndLineFixedIsOptIn(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	pid, _, _ := seedProduct(t, db, "SKU-FIX")
	seedDiscount(t, db, model.ProductDiscount{
		ProductID: pid, DiscountType: "FIXED", PerItem: true, Value: 1500, MinQty: 6,
	})
	pid2, _, _ := seedProduct(t, db, "SKU-LINE")
	seedDiscount(t, db, model.ProductDiscount{
		ProductID: pid2, DiscountType: "FIXED", PerItem: false, Value: 6000, MinQty: 6,
	})

	// Default: the per-item rule converts, the line-level one is reported.
	got := plan(t, db, grosirmigrate.Options{})
	conv := got.Convertible()
	require.Len(t, conv, 1)
	require.Equal(t, int64(8500), conv[0].TierPrice) // 10.000 − 1.500 per item
	skipped := got.Skipped()
	require.Len(t, skipped, 1)
	require.Contains(t, skipped[0].Reason, "include-line-fixed")

	// Opted in: the line amount is spread over the threshold (6.000 / 6 = 1.000).
	got = plan(t, db, grosirmigrate.Options{IncludeLineFixed: true})
	require.Len(t, got.Convertible(), 2)
	require.Empty(t, got.Skipped())
	for _, it := range got.Convertible() {
		if it.ProductID == pid2 {
			require.Equal(t, int64(9000), it.TierPrice)
		}
	}
}

func TestBuildPlan_SkipsNonQtyGatedExpiredAndInertRows(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	pid, _, _ := seedProduct(t, db, "SKU-SKIP")
	yesterday := time.Now().AddDate(0, 0, -1)

	seedDiscount(t, db, model.ProductDiscount{ // always-on: no tier form
		ProductID: pid, DiscountType: "PERCENT", Value: 500, MinQty: 0,
	})
	seedDiscount(t, db, model.ProductDiscount{ // min_qty 1 is still not a tier
		ProductID: pid, DiscountType: "PERCENT", Value: 500, MinQty: 1,
	})
	seedDiscount(t, db, model.ProductDiscount{
		ProductID: pid, DiscountType: "PERCENT", Value: 500, MinQty: 4, ExpiresAt: &yesterday,
	})
	seedDiscount(t, db, model.ProductDiscount{ // 0% off changes nothing
		ProductID: pid, DiscountType: "PERCENT", Value: 0, MinQty: 5,
	})

	got := plan(t, db, grosirmigrate.Options{})
	require.Empty(t, got.Convertible())
	require.Len(t, got.Skipped(), 4)
	reasons := ""
	for _, it := range got.Skipped() {
		reasons += it.Reason + "\n"
	}
	require.Contains(t, reasons, "not qty-gated")
	require.Contains(t, reasons, "expired")
	require.Contains(t, reasons, "no price reduction")
}

func TestBuildPlan_SkipsWhenTierAlreadyExists(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	pid, baseID, _ := seedProduct(t, db, "SKU-DUP")
	require.NoError(t, db.Create(&model.ProductPriceTier{
		ProductID: pid, ProductUnitID: baseID, UnitName: "pcs", UnitFactor: 1, MinQty: 12, Price: 9500,
	}).Error)
	seedDiscount(t, db, model.ProductDiscount{
		ProductID: pid, DiscountType: "PERCENT", Value: 1000, MinQty: 12,
	})

	got := plan(t, db, grosirmigrate.Options{})
	require.Empty(t, got.Convertible())
	require.Len(t, got.Skipped(), 1)
	require.Contains(t, got.Skipped()[0].Reason, "already exists")
}

// Two discounts on the same unit and threshold cannot both become tiers — the
// unique index forbids it. The second must be reported, not blow up Apply.
func TestBuildPlan_SkipsSecondDiscountCollidingWithinTheSameRun(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	pid, _, _ := seedProduct(t, db, "SKU-COLLIDE")
	seedDiscount(t, db, model.ProductDiscount{ProductID: pid, DiscountType: "PERCENT", Value: 1000, MinQty: 12})
	seedDiscount(t, db, model.ProductDiscount{ProductID: pid, DiscountType: "PERCENT", Value: 2000, MinQty: 12})

	got := plan(t, db, grosirmigrate.Options{})
	require.Len(t, got.Convertible(), 1)
	require.Len(t, got.Skipped(), 1)
	require.Contains(t, got.Skipped()[0].Reason, "already exists")

	n, err := grosirmigrate.Apply(db, got)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

func TestBuildPlan_SkipsArchivedAndNonSellableUnits(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	pid, _, boxID := seedProduct(t, db, "SKU-UNIT")
	require.NoError(t, db.Model(&model.ProductUnit{}).Where("id = ?", boxID).
		Update("active", false).Error)
	seedDiscount(t, db, model.ProductDiscount{
		ProductID: pid, DiscountType: "PERCENT", Value: 1000, MinQty: 3,
		MinQtyUnitID: &boxID, MinQtyUnitName: "box", MinQtyUnitFactor: 12,
	})

	got := plan(t, db, grosirmigrate.Options{})
	require.Empty(t, got.Convertible())
	require.Len(t, got.Skipped(), 1)
	require.Contains(t, got.Skipped()[0].Reason, "archived")
}

func TestApply_WritesTiersAndDeletesConvertedDiscounts(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	pid, baseID, _ := seedProduct(t, db, "SKU-APPLY")
	converted := seedDiscount(t, db, model.ProductDiscount{
		ProductID: pid, DiscountType: "PERCENT", Value: 1000, MinQty: 12,
	})
	kept := seedDiscount(t, db, model.ProductDiscount{ // not qty-gated => untouched
		ProductID: pid, DiscountType: "PERCENT", Value: 500, MinQty: 0,
	})

	n, err := grosirmigrate.Apply(db, plan(t, db, grosirmigrate.Options{}))
	require.NoError(t, err)
	require.Equal(t, 1, n)

	var tiers []model.ProductPriceTier
	require.NoError(t, db.Where("product_id = ?", pid).Find(&tiers).Error)
	require.Len(t, tiers, 1)
	require.Equal(t, baseID, tiers[0].ProductUnitID)
	require.Equal(t, "pcs", tiers[0].UnitName)
	require.Equal(t, int32(12), tiers[0].MinQty)
	require.Equal(t, int64(9000), tiers[0].Price)

	var remaining []model.ProductDiscount
	require.NoError(t, db.Where("product_id = ?", pid).Find(&remaining).Error)
	require.Len(t, remaining, 1)
	require.Equal(t, kept, remaining[0].ID)
	require.NotEqual(t, converted, remaining[0].ID)
}

// A second run has nothing left to do: the sources are gone and the tiers it
// wrote are now the "already exists" guard.
func TestApply_IsIdempotent(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	pid, _, _ := seedProduct(t, db, "SKU-TWICE")
	seedDiscount(t, db, model.ProductDiscount{
		ProductID: pid, DiscountType: "PERCENT", Value: 1000, MinQty: 12,
	})

	n, err := grosirmigrate.Apply(db, plan(t, db, grosirmigrate.Options{}))
	require.NoError(t, err)
	require.Equal(t, 1, n)

	second := plan(t, db, grosirmigrate.Options{})
	require.Empty(t, second.Items)
	n, err = grosirmigrate.Apply(db, second)
	require.NoError(t, err)
	require.Zero(t, n)

	var tiers int64
	require.NoError(t, db.Model(&model.ProductPriceTier{}).Where("product_id = ?", pid).Count(&tiers).Error)
	require.Equal(t, int64(1), tiers)
}

// A converted tier must start its price history like any other, or the ladder
// reads as if its price appeared from nowhere on the first UI edit.
func TestApply_OpensTierPriceHistory(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	pid, _, _ := seedProduct(t, db, "SKU-HIST")
	seedDiscount(t, db, model.ProductDiscount{
		ProductID: pid, DiscountType: "PERCENT", Value: 1000, MinQty: 12,
	})

	n, err := grosirmigrate.Apply(db, plan(t, db, grosirmigrate.Options{}))
	require.NoError(t, err)
	require.Equal(t, 1, n)

	var tier model.ProductPriceTier
	require.NoError(t, db.Where("product_id = ?", pid).First(&tier).Error)

	var rows []model.ProductTierPrice
	require.NoError(t, db.Where("product_id = ?", pid).Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, tier.ProductUnitID, rows[0].ProductUnitID)
	require.Equal(t, tier.MinQty, rows[0].MinQty)
	require.Equal(t, tier.Price, rows[0].Price)
	require.Nil(t, rows[0].EffectiveTo, "the rung is open")
	require.Nil(t, rows[0].ChangedBy, "a system conversion has no user")
}

func TestBuildPlan_EmptyDatabase(t *testing.T) {
	t.Parallel()
	got := plan(t, newDB(t), grosirmigrate.Options{})
	require.Empty(t, got.Items)
	require.Empty(t, got.Convertible())
	require.Empty(t, got.Skipped())
}

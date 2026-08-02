package sale_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// seedKindProduct inserts a product of an explicit kind plus its base unit (the
// sale service's resolveSellUnit needs one for every sellable product, composite
// and service included — a menu item is still priced and sold in a unit).
func seedKindProduct(t *testing.T, db *gorm.DB, sku, name string, unitPrice int64, kind string) string {
	t.Helper()
	p := model.Product{
		SKU:       sku,
		Name:      name,
		Unit:      "porsi",
		UnitPrice: unitPrice,
		Active:    true,
		Kind:      kind,
	}
	require.NoError(t, db.Create(&p).Error)
	require.NoError(t, db.Create(&model.ProductUnit{
		ProductID: p.ID,
		Name:      "porsi",
		Factor:    1,
		IsBase:    true,
		SellPrice: unitPrice,
		Sellable:  true,
	}).Error)
	return p.ID
}

// seedRecipe adds one ingredient line to a composite's recipe directly (the
// sale tests exercise the explosion, not the recipe CRUD, which has its own
// co-located tests).
func seedRecipe(t *testing.T, db *gorm.DB, parentID, componentID string, qtyBase int64) {
	t.Helper()
	require.NoError(t, db.Create(&model.ProductRecipeItem{
		ProductID:          parentID,
		ComponentProductID: componentID,
		QtyBase:            qtyBase,
	}).Error)
}

// stockOnHand is the current on-hand for a product in MAIN, summed straight off
// the ledger — the same definition every read path uses.
func stockOnHand(t *testing.T, db *gorm.DB, productID string) int64 {
	t.Helper()
	var qty int64
	require.NoError(t, db.
		Table("batches AS b").
		Select("COALESCE(SUM(sm.qty), 0)").
		Joins("LEFT JOIN stock_movements sm ON sm.batch_id = b.id AND sm.warehouse_id = ?", mainWarehouseID).
		Where("b.product_id = ?", productID).
		Scan(&qty).Error)
	return qty
}

// compositeKind / stockedKind / serviceKind alias the shared constants so the
// tests read as intent rather than string literals.
const (
	compositeKind = common.ProductKindComposite
	serviceKind   = common.ProductKindService
)

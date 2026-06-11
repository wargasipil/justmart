package sale_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// setPharmacyMode flips the shop into pharmacy mode so the POS Rx gate is active.
// Without this the default business mode is UNSPECIFIED (retail), and Rx
// enforcement is a no-op (see assertRxCovers).
func setPharmacyMode(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, common.SetBussinessType(context.Background(), db, common.BussinessTypePharmacyShop))
}

// setRetailMode flips the shop into explicit retail mode (the Rx gate is off).
func setRetailMode(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, common.SetBussinessType(context.Background(), db, common.BussinessTypeRetail))
}

// seedRxProduct is seedProduct + prescription_required=true (the pharmacy gate
// only fires for these). Returns the product id.
func seedRxProduct(t *testing.T, db *gorm.DB, sku, name string, unitPrice int64) string {
	t.Helper()
	p := model.Product{
		SKU:                  sku,
		Name:                 name,
		Unit:                 "tab",
		UnitPrice:            unitPrice,
		PrescriptionRequired: true,
		Active:               true,
	}
	require.NoError(t, db.Create(&p).Error)
	base := model.ProductUnit{
		ProductID: p.ID,
		Name:      "tab",
		Factor:    1,
		IsBase:    true,
		SellPrice: unitPrice,
		Sellable:  true,
	}
	require.NoError(t, db.Create(&base).Error)
	return p.ID
}

func seedCustomer(t *testing.T, db *gorm.DB, name string) string {
	t.Helper()
	c := model.Customer{Name: name}
	require.NoError(t, db.Create(&c).Error)
	return c.ID
}

// seedPrescription creates an ACTIVE prescription for one product with the given
// prescribed qty (base units). createdBy must be a real users.id (FK).
func seedPrescription(t *testing.T, db *gorm.DB, customerID, createdBy, productID string, prescribedQty int32) string {
	t.Helper()
	now := time.Now()
	rx := model.Prescription{
		CustomerID: customerID,
		IssuerName: "dr. Test",
		IssuedAt:   now,
		ExpiresAt:  now.AddDate(0, 0, 90),
		Status:     "ACTIVE",
		CreatedBy:  createdBy,
	}
	require.NoError(t, db.Create(&rx).Error)
	item := model.PrescriptionItem{
		PrescriptionID: rx.ID,
		ProductID:      productID,
		PrescribedQty:  prescribedQty,
		DispensedQty:   0,
	}
	require.NoError(t, db.Create(&item).Error)
	return rx.ID
}

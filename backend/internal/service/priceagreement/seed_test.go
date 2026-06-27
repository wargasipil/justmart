package priceagreement_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	priceagreementsvc "github.com/justmart/backend/internal/service/priceagreement"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/servicetest"
)

func newSvc(t *testing.T) (*priceagreementsvc.PriceAgreementService, *gorm.DB) {
	t.Helper()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	return priceagreementsvc.NewPriceAgreementService(db), db
}

func seedSupplier(t *testing.T, db *gorm.DB, code string) string {
	t.Helper()
	s := model.Supplier{Code: code, Name: code + " Co", Active: true}
	require.NoError(t, db.Create(&s).Error)
	return s.ID
}

// seedProductWithUnits inserts a product + its base "pcs" unit + a "box" (×12)
// purchasable unit. Returns (productID, baseUnitID, boxUnitID).
func seedProductWithUnits(t *testing.T, db *gorm.DB, sku string) (string, string, string) {
	t.Helper()
	p := model.Product{SKU: sku, Name: sku + " Item", Unit: "pcs", UnitPrice: 1000, Active: true}
	require.NoError(t, db.Create(&p).Error)
	base := model.ProductUnit{ProductID: p.ID, Name: "pcs", Factor: 1, IsBase: true, SellPrice: 1000, Sellable: true, Purchasable: true, Active: true}
	require.NoError(t, db.Create(&base).Error)
	box := model.ProductUnit{ProductID: p.ID, Name: "box", Factor: 12, SellPrice: 11000, Sellable: true, Purchasable: true, Active: true}
	require.NoError(t, db.Create(&box).Error)
	return p.ID, base.ID, box.ID
}

func ctx() context.Context { return context.Background() }

// requireToken asserts a connect error with the given code + stable token message.
func requireToken(t *testing.T, err error, code connect.Code, token string) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, code, connect.CodeOf(err))
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	require.Equal(t, token, ce.Message())
}

package productpricetier_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	productpricetiersvc "github.com/justmart/backend/internal/service/productpricetier"
	"github.com/justmart/backend/internal/service/servicetest"
)

func newSvc(t *testing.T) (*productpricetiersvc.ProductPriceTierService, *gorm.DB) {
	t.Helper()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	return productpricetiersvc.NewProductPriceTierService(db), db
}

// seedProductWithUnits inserts a product with a base "pcs" unit and a larger
// "box" (×12) unit, returning (productID, baseUnitID, boxUnitID). A tier always
// targets a specific unit, so every test needs at least one.
func seedProductWithUnits(t *testing.T, db *gorm.DB, sku string) (string, string, string) {
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

func ctx() context.Context { return context.Background() }

func create(t *testing.T, svc *productpricetiersvc.ProductPriceTierService, productID, unitID string, minQty int32, price int64) *inventoryifacev1.ProductPriceTier {
	t.Helper()
	r, err := svc.CreateProductPriceTier(ctx(), connect.NewRequest(&inventoryifacev1.CreateProductPriceTierRequest{
		ProductId: productID, ProductUnitId: unitID, MinQty: minQty, Price: price,
	}))
	require.NoError(t, err)
	return r.Msg.Tier
}

func requireToken(t *testing.T, err error, code connect.Code, token string) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, code, connect.CodeOf(err))
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	require.Equal(t, token, ce.Message())
}

package productdiscount_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	productdiscountsvc "github.com/justmart/backend/internal/service/productdiscount"
	"github.com/justmart/backend/internal/service/servicetest"
)

func newSvc(t *testing.T) (*productdiscountsvc.ProductDiscountService, *gorm.DB) {
	t.Helper()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	return productdiscountsvc.NewProductDiscountService(db), db
}

func seedProduct(t *testing.T, db *gorm.DB, sku string) string {
	t.Helper()
	p := model.Product{SKU: sku, Name: sku + " Item", Unit: "pcs", UnitPrice: 1000, Active: true}
	require.NoError(t, db.Create(&p).Error)
	return p.ID
}

func ctx() context.Context { return context.Background() }

func create(t *testing.T, svc *productdiscountsvc.ProductDiscountService, productID, typ string, perItem bool, value int64, minQty int32) *inventoryifacev1.ProductDiscount {
	t.Helper()
	r, err := svc.CreateProductDiscount(ctx(), connect.NewRequest(&inventoryifacev1.CreateProductDiscountRequest{
		ProductId: productID, DiscountType: typ, PerItem: perItem, Value: value, MinQty: minQty,
	}))
	require.NoError(t, err)
	return r.Msg.Discount
}

func requireToken(t *testing.T, err error, code connect.Code, token string) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, code, connect.CodeOf(err))
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	require.Equal(t, token, ce.Message())
}

package productdiscount_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func TestCreateProductDiscount_Modes(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	p := seedProduct(t, db, "cpd-1")

	d := create(t, svc, p, "PERCENT", false, 1000, 3) // 10%, buy ≥3
	require.Equal(t, "PERCENT", d.DiscountType)
	require.False(t, d.PerItem)
	require.EqualValues(t, 1000, d.Value)
	require.EqualValues(t, 3, d.MinQty)
	require.NotEmpty(t, d.Id)

	d2 := create(t, svc, p, "FIXED", true, 500, 0) // Rp500 off per item
	require.Equal(t, "FIXED", d2.DiscountType)
	require.True(t, d2.PerItem)
	require.EqualValues(t, 500, d2.Value)
}

func TestCreateProductDiscount_MinQtyUnit(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	p := seedProduct(t, db, "cpdu-1")
	box := model.ProductUnit{ProductID: p, Name: "box", Factor: 12, SellPrice: 11000, Sellable: true, Active: true}
	require.NoError(t, db.Create(&box).Error)

	r, err := svc.CreateProductDiscount(ctx(), connect.NewRequest(&inventoryifacev1.CreateProductDiscountRequest{
		ProductId: p, DiscountType: "PERCENT", Value: 1000, MinQty: 3, MinQtyUnitId: box.ID,
	}))
	require.NoError(t, err)
	require.Equal(t, box.ID, r.Msg.Discount.MinQtyUnitId)
	require.Equal(t, "box", r.Msg.Discount.MinQtyUnitName)
	require.EqualValues(t, 12, r.Msg.Discount.MinQtyUnitFactor)

	// Update can switch back to the base unit ("" clears the unit).
	u, err := svc.UpdateProductDiscount(ctx(), connect.NewRequest(&inventoryifacev1.UpdateProductDiscountRequest{
		Id: r.Msg.Discount.Id, DiscountType: "PERCENT", Value: 1000, MinQty: 3, MinQtyUnitId: "",
	}))
	require.NoError(t, err)
	require.Equal(t, "", u.Msg.Discount.MinQtyUnitId)
	require.EqualValues(t, 1, u.Msg.Discount.MinQtyUnitFactor)

	// A unit belonging to a DIFFERENT product is rejected.
	p2 := seedProduct(t, db, "cpdu-2")
	_, err = svc.CreateProductDiscount(ctx(), connect.NewRequest(&inventoryifacev1.CreateProductDiscountRequest{
		ProductId: p2, DiscountType: "FIXED", Value: 500, MinQty: 1, MinQtyUnitId: box.ID,
	}))
	requireToken(t, err, connect.CodeFailedPrecondition, "product_discount.unit_invalid")
}

func TestCreateProductDiscount_Validation(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	p := seedProduct(t, db, "cpd-2")

	mk := func(req *inventoryifacev1.CreateProductDiscountRequest) error {
		_, err := svc.CreateProductDiscount(ctx(), connect.NewRequest(req))
		return err
	}

	// non-existent product
	requireToken(t, mk(&inventoryifacev1.CreateProductDiscountRequest{ProductId: "00000000-0000-0000-0000-000000000000", DiscountType: "PERCENT", Value: 100}),
		connect.CodeFailedPrecondition, "product_discount.product_missing")
	// negative value
	requireToken(t, mk(&inventoryifacev1.CreateProductDiscountRequest{ProductId: p, DiscountType: "FIXED", Value: -1}),
		connect.CodeInvalidArgument, "product_discount.value_invalid")
	// percent over 100%
	requireToken(t, mk(&inventoryifacev1.CreateProductDiscountRequest{ProductId: p, DiscountType: "PERCENT", Value: 10001}),
		connect.CodeInvalidArgument, "product_discount.value_invalid")
	// bad type
	requireToken(t, mk(&inventoryifacev1.CreateProductDiscountRequest{ProductId: p, DiscountType: "WHAT", Value: 1}),
		connect.CodeInvalidArgument, "product_discount.type_invalid")
	// negative threshold
	requireToken(t, mk(&inventoryifacev1.CreateProductDiscountRequest{ProductId: p, DiscountType: "FIXED", Value: 1, MinQty: -1}),
		connect.CodeInvalidArgument, "product_discount.threshold_invalid")
	// bad expiry
	requireToken(t, mk(&inventoryifacev1.CreateProductDiscountRequest{ProductId: p, DiscountType: "FIXED", Value: 1, ExpiresAt: "2026/01/01"}),
		connect.CodeInvalidArgument, "product_discount.bad_expiry")
}

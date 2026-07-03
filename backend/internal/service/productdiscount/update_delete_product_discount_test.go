package productdiscount_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestUpdateProductDiscount(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	p := seedProduct(t, db, "upd-1")
	d := create(t, svc, p, "PERCENT", false, 1000, 0)

	r, err := svc.UpdateProductDiscount(ctx(), connect.NewRequest(&inventoryifacev1.UpdateProductDiscountRequest{
		Id: d.Id, DiscountType: "FIXED", PerItem: true, Value: 750, MinQty: 5, ExpiresAt: "2026-12-31",
	}))
	require.NoError(t, err)
	require.Equal(t, "FIXED", r.Msg.Discount.DiscountType)
	require.True(t, r.Msg.Discount.PerItem)
	require.EqualValues(t, 750, r.Msg.Discount.Value)
	require.EqualValues(t, 5, r.Msg.Discount.MinQty)
	require.Equal(t, "2026-12-31", r.Msg.Discount.ExpiresAt)
}

func TestDeleteProductDiscount(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	p := seedProduct(t, db, "del-1")
	d := create(t, svc, p, "PERCENT", false, 1000, 0)

	_, err := svc.DeleteProductDiscount(ctx(), connect.NewRequest(&inventoryifacev1.DeleteProductDiscountRequest{Id: d.Id}))
	require.NoError(t, err)

	r, err := svc.ListProductDiscounts(ctx(), connect.NewRequest(&inventoryifacev1.ListProductDiscountsRequest{ProductId: p}))
	require.NoError(t, err)
	require.Empty(t, r.Msg.Discounts)
}

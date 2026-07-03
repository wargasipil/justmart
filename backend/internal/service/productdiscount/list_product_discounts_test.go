package productdiscount_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestListProductDiscounts(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	p1 := seedProduct(t, db, "lpd-1")
	p2 := seedProduct(t, db, "lpd-2")
	create(t, svc, p1, "PERCENT", false, 1000, 0)
	create(t, svc, p1, "FIXED", true, 500, 3)
	create(t, svc, p2, "PERCENT", false, 2000, 0)

	r, err := svc.ListProductDiscounts(ctx(), connect.NewRequest(&inventoryifacev1.ListProductDiscountsRequest{ProductId: p1}))
	require.NoError(t, err)
	require.Len(t, r.Msg.Discounts, 2)
	for _, d := range r.Msg.Discounts {
		require.Equal(t, p1, d.ProductId)
	}

	r2, err := svc.ListProductDiscounts(ctx(), connect.NewRequest(&inventoryifacev1.ListProductDiscountsRequest{ProductId: p2}))
	require.NoError(t, err)
	require.Len(t, r2.Msg.Discounts, 1)
}

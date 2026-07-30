package productpricetier_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

// The Grosir tab groups by unit (base first, then by factor) and shows each
// ladder ascending, so the list must arrive in that order regardless of insert
// order.
func TestListProductPriceTiers_OrdersBaseFirstThenAscending(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	prodID, baseID, boxID := seedProductWithUnits(t, db, "PT-L1")
	// Insert deliberately out of order.
	create(t, svc, prodID, boxID, 5, 100000)
	create(t, svc, prodID, baseID, 144, 7200)
	create(t, svc, prodID, baseID, 12, 8500)
	create(t, svc, prodID, baseID, 60, 8000)

	resp, err := svc.ListProductPriceTiers(ctx(), connect.NewRequest(&inventoryifacev1.ListProductPriceTiersRequest{
		ProductId: prodID,
	}))
	require.NoError(t, err)
	tiers := resp.Msg.Tiers
	require.Len(t, tiers, 4)

	require.Equal(t, "pcs", tiers[0].UnitName)
	require.Equal(t, int32(12), tiers[0].MinQty)
	require.Equal(t, int32(60), tiers[1].MinQty)
	require.Equal(t, int32(144), tiers[2].MinQty)
	require.Equal(t, "box", tiers[3].UnitName)
	require.Equal(t, int32(5), tiers[3].MinQty)
}

func TestListProductPriceTiers_ScopedToProduct(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	prodA, baseA, _ := seedProductWithUnits(t, db, "PT-L2a")
	prodB, baseB, _ := seedProductWithUnits(t, db, "PT-L2b")
	create(t, svc, prodA, baseA, 12, 8500)
	create(t, svc, prodB, baseB, 20, 9000)

	resp, err := svc.ListProductPriceTiers(ctx(), connect.NewRequest(&inventoryifacev1.ListProductPriceTiersRequest{
		ProductId: prodA,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Tiers, 1)
	require.Equal(t, int32(12), resp.Msg.Tiers[0].MinQty)
}

func TestListProductPriceTiers_EmptyAndMissingProduct(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	prodID, _, _ := seedProductWithUnits(t, db, "PT-L3")

	resp, err := svc.ListProductPriceTiers(ctx(), connect.NewRequest(&inventoryifacev1.ListProductPriceTiersRequest{
		ProductId: prodID,
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Tiers)

	_, err = svc.ListProductPriceTiers(ctx(), connect.NewRequest(&inventoryifacev1.ListProductPriceTiersRequest{
		ProductId: "",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

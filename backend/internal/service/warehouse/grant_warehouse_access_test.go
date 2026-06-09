package warehouse_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
	warehousesvc "github.com/justmart/backend/internal/service/warehouse"
)

func TestGrantWarehouseAccess_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := warehousesvc.NewWarehouseService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Create a second warehouse to grant the owner access to. The auto-grant on
	// CreateWarehouse inserts a non-default membership; grant it again as a
	// non-default membership (is_default stays on MAIN, which EnsureOwner set —
	// the partial unique index allows only one default row per user).
	wh, err := svc.CreateWarehouse(ctx, connect.NewRequest(&warehouseifacev1.CreateWarehouseRequest{
		Code: "GRT1", Name: "Grantee WH",
	}))
	require.NoError(t, err)
	whID := wh.Msg.Warehouse.Id

	resp, err := svc.GrantWarehouseAccess(ctx, connect.NewRequest(&warehouseifacev1.GrantWarehouseAccessRequest{
		UserId:      ownerID,
		WarehouseId: whID,
		IsDefault:   false,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Membership)
	require.Equal(t, ownerID, resp.Msg.Membership.UserId)
	require.Equal(t, whID, resp.Msg.Membership.WarehouseId)
	require.False(t, resp.Msg.Membership.IsDefault)

	// The owner now has access to both MAIN and the new warehouse, with exactly
	// one default (MAIN) intact.
	list, err := svc.ListUserWarehouses(ctx, connect.NewRequest(&warehouseifacev1.ListUserWarehousesRequest{
		UserId: ownerID,
	}))
	require.NoError(t, err)
	require.Len(t, list.Msg.Memberships, 2)
	defaults := 0
	for _, m := range list.Msg.Memberships {
		if m.IsDefault {
			defaults++
			require.Equal(t, mainWarehouseID, m.WarehouseId)
		}
	}
	require.Equal(t, 1, defaults)
}

func TestGrantWarehouseAccess_MissingArgs(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.GrantWarehouseAccess(context.Background(), connect.NewRequest(&warehouseifacev1.GrantWarehouseAccessRequest{
		UserId:      "",
		WarehouseId: "",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

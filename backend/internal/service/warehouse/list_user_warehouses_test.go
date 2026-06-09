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

func TestListUserWarehouses_Self(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg) // grants the owner the MAIN membership
	svc := warehousesvc.NewWarehouseService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Empty user_id -> self. Owner has MAIN granted by EnsureBootstrapOwner.
	resp, err := svc.ListUserWarehouses(ctx, connect.NewRequest(&warehouseifacev1.ListUserWarehousesRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Memberships, 1)
	require.Equal(t, ownerID, resp.Msg.Memberships[0].UserId)
	require.Equal(t, mainWarehouseID, resp.Msg.Memberships[0].WarehouseId)
	// Warehouses are hydrated alongside the memberships.
	require.Len(t, resp.Msg.Warehouses, 1)
	require.Equal(t, "MAIN", resp.Msg.Warehouses[0].Code)
}

func TestListUserWarehouses_OtherUserForbiddenForNonOwner(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := warehousesvc.NewWarehouseService(gormDB)

	// A CASHIER asking for someone else's memberships -> PermissionDenied.
	ctx := servicetest.CtxAs(context.Background(), "CASHIER", "00000000-0000-0000-0000-0000cashier01")
	_, err := svc.ListUserWarehouses(ctx, connect.NewRequest(&warehouseifacev1.ListUserWarehousesRequest{
		UserId: ownerID, // different from the caller
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestListUserWarehouses_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.ListUserWarehouses(context.Background(), connect.NewRequest(&warehouseifacev1.ListUserWarehousesRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

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

func TestSetDefaultWarehouse_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg) // owner has the MAIN membership
	svc := warehousesvc.NewWarehouseService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Empty user_id -> self. Owner already has access to MAIN.
	resp, err := svc.SetDefaultWarehouse(ctx, connect.NewRequest(&warehouseifacev1.SetDefaultWarehouseRequest{
		WarehouseId: mainWarehouseID,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Membership)
	require.Equal(t, ownerID, resp.Msg.Membership.UserId)
	require.Equal(t, mainWarehouseID, resp.Msg.Membership.WarehouseId)
	require.True(t, resp.Msg.Membership.IsDefault)
}

func TestSetDefaultWarehouse_NoAccess(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := warehousesvc.NewWarehouseService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Owner has no membership for this (nonexistent) warehouse -> PermissionDenied.
	_, err := svc.SetDefaultWarehouse(ctx, connect.NewRequest(&warehouseifacev1.SetDefaultWarehouseRequest{
		WarehouseId: "00000000-0000-0000-0000-00000000dead",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestSetDefaultWarehouse_OtherUserForbiddenForNonOwner(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	// A PHARMACIST setting another user's default -> PermissionDenied.
	ctx := servicetest.CtxAs(context.Background(), "PHARMACIST", "00000000-0000-0000-0000-0000pharm0001")
	_, err := svc.SetDefaultWarehouse(ctx, connect.NewRequest(&warehouseifacev1.SetDefaultWarehouseRequest{
		UserId:      "00000000-0000-0000-0000-0000someoneelse",
		WarehouseId: mainWarehouseID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestSetDefaultWarehouse_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.SetDefaultWarehouse(context.Background(), connect.NewRequest(&warehouseifacev1.SetDefaultWarehouseRequest{
		WarehouseId: mainWarehouseID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

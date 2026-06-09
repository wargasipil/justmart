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

func TestRevokeWarehouseAccess_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg) // grants MAIN membership
	svc := warehousesvc.NewWarehouseService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Grant access to a fresh warehouse, then revoke it.
	wh, err := svc.CreateWarehouse(ctx, connect.NewRequest(&warehouseifacev1.CreateWarehouseRequest{
		Code: "RVK1", Name: "Revoke WH",
	}))
	require.NoError(t, err)
	whID := wh.Msg.Warehouse.Id
	// CreateWarehouse auto-granted the owner; revoke that membership.
	_, err = svc.RevokeWarehouseAccess(ctx, connect.NewRequest(&warehouseifacev1.RevokeWarehouseAccessRequest{
		UserId:      ownerID,
		WarehouseId: whID,
	}))
	require.NoError(t, err)

	// Revoking again -> the membership no longer exists -> NotFound.
	_, err = svc.RevokeWarehouseAccess(ctx, connect.NewRequest(&warehouseifacev1.RevokeWarehouseAccessRequest{
		UserId:      ownerID,
		WarehouseId: whID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestRevokeWarehouseAccess_NotFound(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.RevokeWarehouseAccess(context.Background(), connect.NewRequest(&warehouseifacev1.RevokeWarehouseAccessRequest{
		UserId:      "00000000-0000-0000-0000-00000000beef",
		WarehouseId: mainWarehouseID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

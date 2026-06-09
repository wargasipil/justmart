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

func TestListWarehouseUsers_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg) // grants the owner the MAIN membership
	svc := warehousesvc.NewWarehouseService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	resp, err := svc.ListWarehouseUsers(ctx, connect.NewRequest(&warehouseifacev1.ListWarehouseUsersRequest{
		WarehouseId: mainWarehouseID,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Users, 1)
	u := resp.Msg.Users[0]
	require.Equal(t, ownerID, u.UserId)
	require.Equal(t, servicetest.OwnerEmail, u.Email)
	require.Equal(t, "OWNER", u.Role)
	require.True(t, u.UserActive)
}

func TestListWarehouseUsers_MissingWarehouseID(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.ListWarehouseUsers(context.Background(), connect.NewRequest(&warehouseifacev1.ListWarehouseUsersRequest{
		WarehouseId: "",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

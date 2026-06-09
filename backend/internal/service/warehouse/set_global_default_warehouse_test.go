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

func TestSetGlobalDefaultWarehouse_Promote(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := warehousesvc.NewWarehouseService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Create a fresh (non-default) warehouse, then promote it to global default.
	created, err := svc.CreateWarehouse(ctx, connect.NewRequest(&warehouseifacev1.CreateWarehouseRequest{
		Code: "GDF1", Name: "New default",
	}))
	require.NoError(t, err)
	newID := created.Msg.Warehouse.Id
	require.False(t, created.Msg.Warehouse.IsDefault)

	resp, err := svc.SetGlobalDefaultWarehouse(ctx, connect.NewRequest(&warehouseifacev1.SetGlobalDefaultWarehouseRequest{
		WarehouseId: newID,
	}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Warehouse.IsDefault)
	require.Equal(t, newID, resp.Msg.Warehouse.Id)

	// The old default (MAIN) was cleared — only one default exists.
	main, err := svc.GetWarehouse(ctx, connect.NewRequest(&warehouseifacev1.GetWarehouseRequest{Id: mainWarehouseID}))
	require.NoError(t, err)
	require.False(t, main.Msg.Warehouse.IsDefault)
}

func TestSetGlobalDefaultWarehouse_AlreadyDefaultNoop(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	// MAIN is already the default -> early return with the same warehouse, no error.
	resp, err := svc.SetGlobalDefaultWarehouse(context.Background(), connect.NewRequest(&warehouseifacev1.SetGlobalDefaultWarehouseRequest{
		WarehouseId: mainWarehouseID,
	}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Warehouse.IsDefault)
	require.Equal(t, mainWarehouseID, resp.Msg.Warehouse.Id)
}

func TestSetGlobalDefaultWarehouse_MissingID(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.SetGlobalDefaultWarehouse(context.Background(), connect.NewRequest(&warehouseifacev1.SetGlobalDefaultWarehouseRequest{
		WarehouseId: "",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSetGlobalDefaultWarehouse_ArchivedRefused(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := warehousesvc.NewWarehouseService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	created, err := svc.CreateWarehouse(ctx, connect.NewRequest(&warehouseifacev1.CreateWarehouseRequest{
		Code: "GDF2", Name: "Soon archived",
	}))
	require.NoError(t, err)
	id := created.Msg.Warehouse.Id
	_, err = svc.ArchiveWarehouse(ctx, connect.NewRequest(&warehouseifacev1.ArchiveWarehouseRequest{Id: id}))
	require.NoError(t, err)

	// Promoting an archived warehouse is refused.
	_, err = svc.SetGlobalDefaultWarehouse(ctx, connect.NewRequest(&warehouseifacev1.SetGlobalDefaultWarehouseRequest{
		WarehouseId: id,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

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

func TestArchiveWarehouse_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := warehousesvc.NewWarehouseService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	created, err := svc.CreateWarehouse(ctx, connect.NewRequest(&warehouseifacev1.CreateWarehouseRequest{
		Code: "ARC1", Name: "To archive",
	}))
	require.NoError(t, err)
	id := created.Msg.Warehouse.Id

	resp, err := svc.ArchiveWarehouse(ctx, connect.NewRequest(&warehouseifacev1.ArchiveWarehouseRequest{Id: id}))
	require.NoError(t, err)
	require.False(t, resp.Msg.Warehouse.Active)
	require.Equal(t, id, resp.Msg.Warehouse.Id)
}

func TestArchiveWarehouse_DefaultRefused(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	// The seeded MAIN warehouse is the default -> archive is refused.
	_, err := svc.ArchiveWarehouse(context.Background(), connect.NewRequest(&warehouseifacev1.ArchiveWarehouseRequest{
		Id: mainWarehouseID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestArchiveWarehouse_NotFound(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.ArchiveWarehouse(context.Background(), connect.NewRequest(&warehouseifacev1.ArchiveWarehouseRequest{
		Id: "00000000-0000-0000-0000-00000000dead",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

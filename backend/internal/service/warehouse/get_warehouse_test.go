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

func TestGetWarehouse_Found(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	// The migration seeds the default MAIN warehouse — no extra seeding needed.
	resp, err := svc.GetWarehouse(context.Background(), connect.NewRequest(&warehouseifacev1.GetWarehouseRequest{
		Id: mainWarehouseID,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Warehouse)
	require.Equal(t, mainWarehouseID, resp.Msg.Warehouse.Id)
	require.Equal(t, "MAIN", resp.Msg.Warehouse.Code)
	require.True(t, resp.Msg.Warehouse.IsDefault)
}

func TestGetWarehouse_NotFound(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.GetWarehouse(context.Background(), connect.NewRequest(&warehouseifacev1.GetWarehouseRequest{
		Id: "00000000-0000-0000-0000-00000000dead",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetWarehouse_EmptyID(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.GetWarehouse(context.Background(), connect.NewRequest(&warehouseifacev1.GetWarehouseRequest{Id: ""}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

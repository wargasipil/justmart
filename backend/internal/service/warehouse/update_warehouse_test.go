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

func TestUpdateWarehouse_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := warehousesvc.NewWarehouseService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	created, err := svc.CreateWarehouse(ctx, connect.NewRequest(&warehouseifacev1.CreateWarehouseRequest{
		Code: "UPD1", Name: "Before",
	}))
	require.NoError(t, err)
	id := created.Msg.Warehouse.Id

	resp, err := svc.UpdateWarehouse(ctx, connect.NewRequest(&warehouseifacev1.UpdateWarehouseRequest{
		Id:      id,
		Name:    "After",
		Address: "Jl. Baru 10",
		Phone:   "021777",
	}))
	require.NoError(t, err)
	w := resp.Msg.Warehouse
	require.Equal(t, id, w.Id)
	require.Equal(t, "After", w.Name)
	require.Equal(t, "Jl. Baru 10", w.Address)
	require.Equal(t, "021777", w.Phone)
	require.Equal(t, "UPD1", w.Code) // code is immutable on update
}

func TestUpdateWarehouse_EmptyName(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	// Target the seeded MAIN warehouse (load succeeds), but blank name -> InvalidArgument.
	_, err := svc.UpdateWarehouse(context.Background(), connect.NewRequest(&warehouseifacev1.UpdateWarehouseRequest{
		Id:   mainWarehouseID,
		Name: "   ",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpdateWarehouse_NotFound(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UpdateWarehouse(context.Background(), connect.NewRequest(&warehouseifacev1.UpdateWarehouseRequest{
		Id:   "00000000-0000-0000-0000-00000000dead",
		Name: "Ghost",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

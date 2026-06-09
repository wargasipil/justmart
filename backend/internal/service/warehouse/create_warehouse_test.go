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

func TestCreateWarehouse_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg) // real users.id for the auto-grant FK
	svc := warehousesvc.NewWarehouseService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	resp, err := svc.CreateWarehouse(ctx, connect.NewRequest(&warehouseifacev1.CreateWarehouseRequest{
		Code:    "wh2", // lower-cased input -> upper-cased on store
		Name:    "Gudang Cabang",
		Address: "Jl. Sudirman 5",
		Phone:   "0218889999",
	}))
	require.NoError(t, err)
	w := resp.Msg.Warehouse
	require.NotNil(t, w)
	require.NotEmpty(t, w.Id) // UUID filled by the SQLite create-callback
	require.Equal(t, "WH2", w.Code)
	require.Equal(t, "Gudang Cabang", w.Name)
	require.Equal(t, "Jl. Sudirman 5", w.Address)
	require.Equal(t, "0218889999", w.Phone)
	require.True(t, w.Active)
	require.False(t, w.IsDefault)
}

func TestCreateWarehouse_MissingCode(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := warehousesvc.NewWarehouseService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.CreateWarehouse(ctx, connect.NewRequest(&warehouseifacev1.CreateWarehouseRequest{
		Code: "   ", // trimmed empty -> InvalidArgument
		Name: "No code",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateWarehouse_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	// No principal in ctx -> auth.MustPrincipal returns CodeUnauthenticated.
	_, err := svc.CreateWarehouse(context.Background(), connect.NewRequest(&warehouseifacev1.CreateWarehouseRequest{
		Code: "WH3",
		Name: "Anon",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

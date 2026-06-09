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

func TestListWarehouses_SeededDefault(t *testing.T) {
	t.Parallel()
	svc := warehousesvc.NewWarehouseService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	// Fresh DB has exactly the migration-seeded MAIN warehouse.
	resp, err := svc.ListWarehouses(context.Background(), connect.NewRequest(&warehouseifacev1.ListWarehousesRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.Len(t, resp.Msg.Warehouses, 1)
	require.Equal(t, "MAIN", resp.Msg.Warehouses[0].Code)
}

func TestListWarehouses_QueryFilterAndPaging(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := warehousesvc.NewWarehouseService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Add two more warehouses so the search filter has something to match.
	for _, c := range []struct{ code, name string }{
		{"BR1", "Cabang Bandung"},
		{"BR2", "Cabang Surabaya"},
	} {
		_, err := svc.CreateWarehouse(ctx, connect.NewRequest(&warehouseifacev1.CreateWarehouseRequest{
			Code: c.code, Name: c.name,
		}))
		require.NoError(t, err)
	}

	// Query by name fragment -> only the matching row(s), total reflects the filter.
	resp, err := svc.ListWarehouses(ctx, connect.NewRequest(&warehouseifacev1.ListWarehousesRequest{
		Query: "Bandung",
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.Len(t, resp.Msg.Warehouses, 1)
	require.Equal(t, "BR1", resp.Msg.Warehouses[0].Code)

	// No filter -> 3 total (MAIN + 2 created); limit caps the page.
	all, err := svc.ListWarehouses(ctx, connect.NewRequest(&warehouseifacev1.ListWarehousesRequest{
		Limit: 2,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(3), all.Msg.Total)
	require.Len(t, all.Msg.Warehouses, 2) // page bounded by limit, total is the unfiltered count
}

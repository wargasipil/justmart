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

// Memberships and warehouses are paged TOGETHER off one join, so the two arrays
// always describe the same rows and `total` respects the `query` filter. Paging
// them separately would emit memberships whose warehouse was filtered out.
func TestListUserWarehouses_Paginates(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg) // MAIN membership
	svc := warehousesvc.NewWarehouseService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// CreateWarehouse auto-grants the creator, so each of these adds a membership.
	for _, c := range []string{"WH-B", "WH-C", "WH-D", "WH-E"} {
		_, err := svc.CreateWarehouse(ctx, connect.NewRequest(&warehouseifacev1.CreateWarehouseRequest{
			Code: c, Name: "Gudang " + c,
		}))
		require.NoError(t, err)
	}

	page := func(limit, offset int32, query string) *warehouseifacev1.ListUserWarehousesResponse {
		t.Helper()
		resp, err := svc.ListUserWarehouses(ctx, connect.NewRequest(
			&warehouseifacev1.ListUserWarehousesRequest{Limit: limit, Offset: offset, Query: query}))
		require.NoError(t, err)
		return resp.Msg
	}

	first := page(2, 0, "")
	require.Equal(t, int32(5), first.Total) // MAIN + 4
	require.Len(t, first.Warehouses, 2)
	// The two arrays are parallel: same length, paired by index.
	require.Len(t, first.Memberships, len(first.Warehouses))
	for i, w := range first.Warehouses {
		require.Equal(t, w.Id, first.Memberships[i].WarehouseId)
		require.Equal(t, ownerID, first.Memberships[i].UserId)
	}

	seen := map[string]bool{}
	for off := int32(0); off < first.Total; off += 2 {
		pg := page(2, off, "")
		require.Equal(t, first.Total, pg.Total)
		require.Len(t, pg.Memberships, len(pg.Warehouses))
		for _, w := range pg.Warehouses {
			require.False(t, seen[w.Id], "warehouse %s appeared on two pages", w.Code)
			seen[w.Id] = true
		}
	}
	require.Len(t, seen, 5)

	// The search filter narrows `total`, not just the page — otherwise a pager
	// would offer pages that don't exist.
	filtered := page(25, 0, "WH-C")
	require.Equal(t, int32(1), filtered.Total)
	require.Len(t, filtered.Warehouses, 1)
	require.Equal(t, "WH-C", filtered.Warehouses[0].Code)
	require.Len(t, filtered.Memberships, 1)
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

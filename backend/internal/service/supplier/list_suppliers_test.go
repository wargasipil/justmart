package supplier_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
	suppliersvc "github.com/justmart/backend/internal/service/supplier"
)

func TestListSuppliers_PaginationAndTotal(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	seedSupplier(t, svc, "LS-A", "Alpha Pharma")
	seedSupplier(t, svc, "LS-B", "Beta Pharma")
	seedSupplier(t, svc, "LS-C", "Gamma Pharma")

	resp, err := svc.ListSuppliers(context.Background(), connect.NewRequest(&inventoryifacev1.ListSuppliersRequest{
		Limit:  2,
		Offset: 0,
	}))
	require.NoError(t, err)
	require.EqualValues(t, 3, resp.Msg.Total) // unfiltered count, not page size
	require.Len(t, resp.Msg.Suppliers, 2)     // page is capped at limit
	// Ordered by name: Alpha, Beta come first.
	require.Equal(t, "Alpha Pharma", resp.Msg.Suppliers[0].Name)
	require.Equal(t, "Beta Pharma", resp.Msg.Suppliers[1].Name)
}

func TestListSuppliers_QueryFilter(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	seedSupplier(t, svc, "QF-1", "Kalbe Farma")
	seedSupplier(t, svc, "QF-2", "Sanbe Farma")
	seedSupplier(t, svc, "QF-3", "Dexa Medica")

	// Match by name.
	resp, err := svc.ListSuppliers(context.Background(), connect.NewRequest(&inventoryifacev1.ListSuppliersRequest{
		Query: "Kalbe",
	}))
	require.NoError(t, err)
	require.EqualValues(t, 1, resp.Msg.Total)
	require.Len(t, resp.Msg.Suppliers, 1)
	require.Equal(t, "Kalbe Farma", resp.Msg.Suppliers[0].Name)

	// Match by code: "QF-2" only matches Sanbe Farma's code (not any name).
	byCode, err := svc.ListSuppliers(context.Background(), connect.NewRequest(&inventoryifacev1.ListSuppliersRequest{
		Query: "QF-2",
	}))
	require.NoError(t, err)
	require.EqualValues(t, 1, byCode.Msg.Total)
	require.Len(t, byCode.Msg.Suppliers, 1)
	require.Equal(t, "Sanbe Farma", byCode.Msg.Suppliers[0].Name)

	// Case-insensitive (pins common.LikeOp: LIKE on SQLite is ASCII-insensitive,
	// ILIKE on Postgres) — lowercase query still matches the mixed-case name.
	ci, err := svc.ListSuppliers(context.Background(), connect.NewRequest(&inventoryifacev1.ListSuppliersRequest{
		Query: "kalbe",
	}))
	require.NoError(t, err)
	require.EqualValues(t, 1, ci.Msg.Total)
	require.Len(t, ci.Msg.Suppliers, 1)
	require.Equal(t, "Kalbe Farma", ci.Msg.Suppliers[0].Name)
}

func TestListSuppliers_ExcludesInactiveByDefault(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := suppliersvc.NewSupplierService(gormDB)
	active := seedSupplier(t, svc, "IN-1", "Active Supplier")
	archived := seedSupplier(t, svc, "IN-2", "Archived Supplier")

	_, err := svc.ArchiveSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.ArchiveSupplierRequest{
		Id: archived.Id,
	}))
	require.NoError(t, err)

	// Default: inactive excluded.
	resp, err := svc.ListSuppliers(context.Background(), connect.NewRequest(&inventoryifacev1.ListSuppliersRequest{}))
	require.NoError(t, err)
	require.EqualValues(t, 1, resp.Msg.Total)
	require.Len(t, resp.Msg.Suppliers, 1)
	require.Equal(t, active.Id, resp.Msg.Suppliers[0].Id)

	// include_inactive=true surfaces both.
	respAll, err := svc.ListSuppliers(context.Background(), connect.NewRequest(&inventoryifacev1.ListSuppliersRequest{
		IncludeInactive: true,
	}))
	require.NoError(t, err)
	require.EqualValues(t, 2, respAll.Msg.Total)
	require.Len(t, respAll.Msg.Suppliers, 2)
}

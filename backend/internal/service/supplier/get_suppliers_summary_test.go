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

func TestGetSuppliersSummary_CountsEveryMatchNotThePage(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	seedSupplier(t, svc, "SM-A", "Alpha Pharma")
	seedSupplier(t, svc, "SM-B", "Beta Pharma")
	seedSupplier(t, svc, "SM-C", "Gamma Pharma")

	resp, err := svc.GetSuppliersSummary(context.Background(),
		connect.NewRequest(&inventoryifacev1.GetSuppliersSummaryRequest{}))
	require.NoError(t, err)
	// The summary takes no paging at all — this is the whole matching set, which
	// is the point of it not being a client-side count of the rendered page.
	require.EqualValues(t, 3, resp.Msg.TotalSuppliers)
}

func TestGetSuppliersSummary_EmptyIsZeroNotAnError(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.GetSuppliersSummary(context.Background(),
		connect.NewRequest(&inventoryifacev1.GetSuppliersSummaryRequest{}))
	require.NoError(t, err)
	require.EqualValues(t, 0, resp.Msg.TotalSuppliers)
}

// The load-bearing property: the tile and the table under it must always
// describe the same set. Both handlers go through supplierFilters; this asserts
// that stays true for every filter combination rather than trusting the shape.
func TestGetSuppliersSummary_HonorsListFilters(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := suppliersvc.NewSupplierService(gormDB)
	seedSupplier(t, svc, "HF-1", "Kalbe Farma")
	seedSupplier(t, svc, "HF-2", "Sanbe Farma")
	seedSupplier(t, svc, "HF-3", "Dexa Medica")
	archived := seedSupplier(t, svc, "HF-4", "Retired Farma")
	_, err := svc.ArchiveSupplier(context.Background(),
		connect.NewRequest(&inventoryifacev1.ArchiveSupplierRequest{Id: archived.Id}))
	require.NoError(t, err)

	cases := []struct {
		name            string
		query           string
		includeInactive bool
	}{
		{name: "no filters", query: "", includeInactive: false},
		{name: "include archived", query: "", includeInactive: true},
		{name: "query by name", query: "Farma", includeInactive: false},
		{name: "query by name incl archived", query: "Farma", includeInactive: true},
		{name: "query by code", query: "HF-2", includeInactive: false},
		{name: "query is case-insensitive", query: "kalbe", includeInactive: false},
		{name: "query matches nothing", query: "zzz-no-such", includeInactive: true},
		{name: "archived-only match needs the flag", query: "Retired", includeInactive: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list, err := svc.ListSuppliers(context.Background(),
				connect.NewRequest(&inventoryifacev1.ListSuppliersRequest{
					Query:           tc.query,
					IncludeInactive: tc.includeInactive,
					// A page size well under the seeded set, so a summary that
					// accidentally counted the page would disagree loudly.
					Limit: 1,
				}))
			require.NoError(t, err)

			summary, err := svc.GetSuppliersSummary(context.Background(),
				connect.NewRequest(&inventoryifacev1.GetSuppliersSummaryRequest{
					Query:           tc.query,
					IncludeInactive: tc.includeInactive,
				}))
			require.NoError(t, err)

			require.EqualValues(t, list.Msg.Total, summary.Msg.TotalSuppliers,
				"summary must count the same set ListSuppliers reports as total")
		})
	}
}

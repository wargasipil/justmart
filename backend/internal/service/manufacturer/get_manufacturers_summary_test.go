package manufacturer_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestGetManufacturersSummary_EmptyIsZeroNotAnError(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)

	resp, err := svc.GetManufacturersSummary(context.Background(),
		connect.NewRequest(&inventoryifacev1.GetManufacturersSummaryRequest{}))
	require.NoError(t, err)
	require.EqualValues(t, 0, resp.Msg.TotalManufacturers)
}

// The load-bearing property: the tile and the table under it must always
// describe the same set. Both handlers go through manufacturerFilters; this
// asserts that stays true for every filter combination rather than trusting the
// shape of the code.
func TestGetManufacturersSummary_HonorsListFilters(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seedManufacturer(t, svc, "SM-A", "Alpha Pharma")
	seedManufacturer(t, svc, "SM-B", "Beta Pharma")
	gone := seedManufacturer(t, svc, "SM-C", "Alpha Kimia")
	_, err := svc.ArchiveManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.ArchiveManufacturerRequest{Id: gone.Id}))
	require.NoError(t, err)

	cases := []struct {
		name            string
		query           string
		includeInactive bool
	}{
		{"no filter", "", false},
		{"with archived", "", true},
		{"query only", "alpha", false},
		{"query + archived", "alpha", true},
		{"query matches nothing", "zzz", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			list, err := svc.ListManufacturers(context.Background(),
				connect.NewRequest(&inventoryifacev1.ListManufacturersRequest{
					Query: tc.query, IncludeInactive: tc.includeInactive, Limit: 1000,
				}))
			require.NoError(t, err)
			summary, err := svc.GetManufacturersSummary(context.Background(),
				connect.NewRequest(&inventoryifacev1.GetManufacturersSummaryRequest{
					Query: tc.query, IncludeInactive: tc.includeInactive,
				}))
			require.NoError(t, err)
			require.EqualValues(t, list.Msg.Total, summary.Msg.TotalManufacturers,
				"summary must count exactly what the list reports")
		})
	}
}

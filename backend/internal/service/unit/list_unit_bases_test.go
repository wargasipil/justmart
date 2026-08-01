package unit_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	unitifacev1 "github.com/justmart/backend/gen/unit_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
	unitsvc "github.com/justmart/backend/internal/service/unit"
)

func TestListUnitBases_Empty(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.ListUnitBases(context.Background(), connect.NewRequest(&unitifacev1.ListUnitBasesRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Bases)
}

// Only BASES page; each base on the page keeps its full derivative set, since a
// base missing derivatives is a broken row rather than a partial page.
func TestListUnitBases_Paginates(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	ctx := context.Background()

	for _, n := range []string{"ampoule", "bottle", "sachet", "tablet", "vial"} {
		_, err := svc.CreateUnitBase(ctx, connect.NewRequest(&unitifacev1.CreateUnitBaseRequest{Name: n}))
		require.NoError(t, err)
	}
	// Give one base a derivative so we can assert the page still hydrates it.
	bases, err := svc.ListUnitBases(ctx, connect.NewRequest(&unitifacev1.ListUnitBasesRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(5), bases.Msg.Total)
	_, err = svc.CreateUnitDerivative(ctx, connect.NewRequest(&unitifacev1.CreateUnitDerivativeRequest{
		BaseUnitId: bases.Msg.Bases[0].Id, Name: "box", Factor: 10,
	}))
	require.NoError(t, err)

	page := func(limit, offset int32) *unitifacev1.ListUnitBasesResponse {
		t.Helper()
		resp, err := svc.ListUnitBases(ctx, connect.NewRequest(&unitifacev1.ListUnitBasesRequest{
			Limit: limit, Offset: offset,
		}))
		require.NoError(t, err)
		return resp.Msg
	}

	first := page(2, 0)
	require.Equal(t, int32(5), first.Total)
	require.Len(t, first.Bases, 2)
	require.Equal(t, "ampoule", first.Bases[0].Name) // name order preserved
	require.Len(t, first.Bases[0].Derivatives, 1)    // hydrated on the page

	seen := map[string]bool{}
	for off := int32(0); off < first.Total; off += 2 {
		pg := page(2, off)
		require.Equal(t, first.Total, pg.Total)
		for _, b := range pg.Bases {
			require.False(t, seen[b.Id], "base %s appeared on two pages", b.Name)
			seen[b.Id] = true
		}
	}
	require.Len(t, seen, 5)

	// An offset past the end returns no bases but keeps the real total.
	last := page(2, 99)
	require.Empty(t, last.Bases)
	require.Equal(t, int32(5), last.Total)
}

func TestListUnitBases_WithDerivativesSortedByName(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	ctx := context.Background()

	// Two bases, inserted out of alphabetical order; ListUnitBases orders by name.
	tabletResp, err := svc.CreateUnitBase(ctx, connect.NewRequest(&unitifacev1.CreateUnitBaseRequest{Name: "tablet"}))
	require.NoError(t, err)
	tabletID := tabletResp.Msg.Base.Id

	_, err = svc.CreateUnitBase(ctx, connect.NewRequest(&unitifacev1.CreateUnitBaseRequest{Name: "bottle"}))
	require.NoError(t, err)

	// A derivative hangs off the tablet base; it should be hydrated on the row.
	_, err = svc.CreateUnitDerivative(ctx, connect.NewRequest(&unitifacev1.CreateUnitDerivativeRequest{
		BaseUnitId: tabletID,
		Name:       "strip",
		Factor:     10,
		SortOrder:  1,
	}))
	require.NoError(t, err)

	resp, err := svc.ListUnitBases(ctx, connect.NewRequest(&unitifacev1.ListUnitBasesRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Bases, 2)
	require.Equal(t, "bottle", resp.Msg.Bases[0].Name)
	require.Equal(t, "tablet", resp.Msg.Bases[1].Name)

	require.Empty(t, resp.Msg.Bases[0].Derivatives)
	require.Len(t, resp.Msg.Bases[1].Derivatives, 1)
	require.Equal(t, "strip", resp.Msg.Bases[1].Derivatives[0].Name)
	require.Equal(t, int64(10), resp.Msg.Bases[1].Derivatives[0].Factor)
}

func TestListUnitBases_ExcludesInactiveByDefault(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	ctx := context.Background()

	created, err := svc.CreateUnitBase(ctx, connect.NewRequest(&unitifacev1.CreateUnitBaseRequest{Name: "tablet"}))
	require.NoError(t, err)
	_, err = svc.ArchiveUnitBase(ctx, connect.NewRequest(&unitifacev1.ArchiveUnitBaseRequest{Id: created.Msg.Base.Id}))
	require.NoError(t, err)

	// Default: archived base hidden.
	active, err := svc.ListUnitBases(ctx, connect.NewRequest(&unitifacev1.ListUnitBasesRequest{}))
	require.NoError(t, err)
	require.Empty(t, active.Msg.Bases)

	// include_inactive surfaces it.
	all, err := svc.ListUnitBases(ctx, connect.NewRequest(&unitifacev1.ListUnitBasesRequest{IncludeInactive: true}))
	require.NoError(t, err)
	require.Len(t, all.Msg.Bases, 1)
	require.False(t, all.Msg.Bases[0].Active)
}

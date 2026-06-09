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

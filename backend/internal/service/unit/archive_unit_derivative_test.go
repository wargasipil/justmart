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

func TestArchiveUnitDerivative_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	ctx := context.Background()
	baseID := newBaseUnit(t, svc, "tablet")

	created, err := svc.CreateUnitDerivative(ctx, connect.NewRequest(&unitifacev1.CreateUnitDerivativeRequest{
		BaseUnitId: baseID,
		Name:       "strip",
		Factor:     10,
	}))
	require.NoError(t, err)
	id := created.Msg.Derivative.Id
	require.True(t, created.Msg.Derivative.Active)

	resp, err := svc.ArchiveUnitDerivative(ctx, connect.NewRequest(&unitifacev1.ArchiveUnitDerivativeRequest{
		Id: id,
	}))
	require.NoError(t, err)
	require.Equal(t, id, resp.Msg.Derivative.Id)
	require.False(t, resp.Msg.Derivative.Active)
}

func TestArchiveUnitDerivative_NotFound(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.ArchiveUnitDerivative(context.Background(), connect.NewRequest(&unitifacev1.ArchiveUnitDerivativeRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

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

func TestUpdateUnitDerivative_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	ctx := context.Background()
	baseID := newBaseUnit(t, svc, "tablet")

	created, err := svc.CreateUnitDerivative(ctx, connect.NewRequest(&unitifacev1.CreateUnitDerivativeRequest{
		BaseUnitId: baseID,
		Name:       "strip",
		Factor:     10,
		SortOrder:  1,
	}))
	require.NoError(t, err)
	id := created.Msg.Derivative.Id

	resp, err := svc.UpdateUnitDerivative(ctx, connect.NewRequest(&unitifacev1.UpdateUnitDerivativeRequest{
		Id:        id,
		Name:      "blister",
		Factor:    12,
		SortOrder: 3,
	}))
	require.NoError(t, err)
	d := resp.Msg.Derivative
	require.Equal(t, id, d.Id)
	require.Equal(t, "blister", d.Name)
	require.Equal(t, int64(12), d.Factor)
	require.Equal(t, int32(3), d.SortOrder)
}

func TestUpdateUnitDerivative_EmptyName(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UpdateUnitDerivative(context.Background(), connect.NewRequest(&unitifacev1.UpdateUnitDerivativeRequest{
		Id:     "any-id",
		Name:   "  ",
		Factor: 10,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpdateUnitDerivative_FactorTooSmall(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UpdateUnitDerivative(context.Background(), connect.NewRequest(&unitifacev1.UpdateUnitDerivativeRequest{
		Id:     "any-id",
		Name:   "strip",
		Factor: 1, // must be > 1, checked before lookup
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpdateUnitDerivative_NotFound(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UpdateUnitDerivative(context.Background(), connect.NewRequest(&unitifacev1.UpdateUnitDerivativeRequest{
		Id:     "00000000-0000-0000-0000-000000000000",
		Name:   "strip",
		Factor: 10,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

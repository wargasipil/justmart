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

// newBaseUnit seeds a base unit and returns its id. Local helper, kept in this
// test file (no shared-fixture edits per the package's test rules).
func newBaseUnit(t *testing.T, svc *unitsvc.UnitService, name string) string {
	t.Helper()
	resp, err := svc.CreateUnitBase(context.Background(), connect.NewRequest(&unitifacev1.CreateUnitBaseRequest{
		Name: name,
	}))
	require.NoError(t, err)
	return resp.Msg.Base.Id
}

func TestCreateUnitDerivative_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	baseID := newBaseUnit(t, svc, "tablet")

	resp, err := svc.CreateUnitDerivative(context.Background(), connect.NewRequest(&unitifacev1.CreateUnitDerivativeRequest{
		BaseUnitId: baseID,
		Name:       "strip",
		Factor:     10,
		SortOrder:  2,
	}))
	require.NoError(t, err)
	d := resp.Msg.Derivative
	require.NotNil(t, d)
	require.NotEmpty(t, d.Id)
	require.Equal(t, baseID, d.BaseUnitId)
	require.Equal(t, "strip", d.Name)
	require.Equal(t, int64(10), d.Factor)
	require.Equal(t, int32(2), d.SortOrder)
	require.True(t, d.Active)
}

func TestCreateUnitDerivative_EmptyName(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	baseID := newBaseUnit(t, svc, "tablet")

	_, err := svc.CreateUnitDerivative(context.Background(), connect.NewRequest(&unitifacev1.CreateUnitDerivativeRequest{
		BaseUnitId: baseID,
		Name:       "  ",
		Factor:     10,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateUnitDerivative_FactorTooSmall(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	baseID := newBaseUnit(t, svc, "tablet")

	_, err := svc.CreateUnitDerivative(context.Background(), connect.NewRequest(&unitifacev1.CreateUnitDerivativeRequest{
		BaseUnitId: baseID,
		Name:       "strip",
		Factor:     1, // must be > 1
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateUnitDerivative_BaseNotFound(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.CreateUnitDerivative(context.Background(), connect.NewRequest(&unitifacev1.CreateUnitDerivativeRequest{
		BaseUnitId: "00000000-0000-0000-0000-000000000000",
		Name:       "strip",
		Factor:     10,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestCreateUnitDerivative_DuplicateActive(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	baseID := newBaseUnit(t, svc, "tablet")

	_, err := svc.CreateUnitDerivative(context.Background(), connect.NewRequest(&unitifacev1.CreateUnitDerivativeRequest{
		BaseUnitId: baseID,
		Name:       "strip",
		Factor:     10,
	}))
	require.NoError(t, err)

	// The (base_unit_id, name) WHERE active=1 unique index rejects this.
	_, err = svc.CreateUnitDerivative(context.Background(), connect.NewRequest(&unitifacev1.CreateUnitDerivativeRequest{
		BaseUnitId: baseID,
		Name:       "strip",
		Factor:     12,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
}

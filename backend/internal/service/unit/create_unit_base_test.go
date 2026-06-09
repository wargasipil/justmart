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

func TestCreateUnitBase_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.CreateUnitBase(context.Background(), connect.NewRequest(&unitifacev1.CreateUnitBaseRequest{
		Name: "tablet",
	}))
	require.NoError(t, err)
	b := resp.Msg.Base
	require.NotNil(t, b)
	require.NotEmpty(t, b.Id) // UUID filled by the SQLite create-callback
	require.Equal(t, "tablet", b.Name)
	require.True(t, b.Active)
	require.Empty(t, b.Derivatives)
}

func TestCreateUnitBase_TrimsName(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.CreateUnitBase(context.Background(), connect.NewRequest(&unitifacev1.CreateUnitBaseRequest{
		Name: "  strip  ",
	}))
	require.NoError(t, err)
	require.Equal(t, "strip", resp.Msg.Base.Name)
}

func TestCreateUnitBase_EmptyName(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.CreateUnitBase(context.Background(), connect.NewRequest(&unitifacev1.CreateUnitBaseRequest{
		Name: "   ", // trimmed to empty -> CodeInvalidArgument
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateUnitBase_DuplicateName(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.CreateUnitBase(context.Background(), connect.NewRequest(&unitifacev1.CreateUnitBaseRequest{
		Name: "box",
	}))
	require.NoError(t, err)

	// The unit_bases.name UNIQUE index rejects the second insert.
	_, err = svc.CreateUnitBase(context.Background(), connect.NewRequest(&unitifacev1.CreateUnitBaseRequest{
		Name: "box",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
}

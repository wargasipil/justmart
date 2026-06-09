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

func TestUpdateUnitBase_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	created, err := svc.CreateUnitBase(context.Background(), connect.NewRequest(&unitifacev1.CreateUnitBaseRequest{
		Name: "tablet",
	}))
	require.NoError(t, err)
	id := created.Msg.Base.Id

	resp, err := svc.UpdateUnitBase(context.Background(), connect.NewRequest(&unitifacev1.UpdateUnitBaseRequest{
		Id:   id,
		Name: "kaplet",
	}))
	require.NoError(t, err)
	require.Equal(t, id, resp.Msg.Base.Id)
	require.Equal(t, "kaplet", resp.Msg.Base.Name)
}

func TestUpdateUnitBase_EmptyName(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UpdateUnitBase(context.Background(), connect.NewRequest(&unitifacev1.UpdateUnitBaseRequest{
		Id:   "any-id",
		Name: "   ", // trimmed to empty -> CodeInvalidArgument (checked before lookup)
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpdateUnitBase_NotFound(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UpdateUnitBase(context.Background(), connect.NewRequest(&unitifacev1.UpdateUnitBaseRequest{
		Id:   "00000000-0000-0000-0000-000000000000",
		Name: "ghost",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

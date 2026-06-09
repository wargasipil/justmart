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

func TestArchiveUnitBase_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	created, err := svc.CreateUnitBase(context.Background(), connect.NewRequest(&unitifacev1.CreateUnitBaseRequest{
		Name: "tablet",
	}))
	require.NoError(t, err)
	id := created.Msg.Base.Id
	require.True(t, created.Msg.Base.Active)

	resp, err := svc.ArchiveUnitBase(context.Background(), connect.NewRequest(&unitifacev1.ArchiveUnitBaseRequest{
		Id: id,
	}))
	require.NoError(t, err)
	require.Equal(t, id, resp.Msg.Base.Id)
	require.False(t, resp.Msg.Base.Active)
}

func TestArchiveUnitBase_NotFound(t *testing.T) {
	t.Parallel()
	svc := unitsvc.NewUnitService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.ArchiveUnitBase(context.Background(), connect.NewRequest(&unitifacev1.ArchiveUnitBaseRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

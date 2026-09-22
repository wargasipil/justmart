package manufacturer_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestGetManufacturer_RoundTrip(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seeded := seedManufacturer(t, svc, "G-1", "PT Sido Muncul")

	resp, err := svc.GetManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.GetManufacturerRequest{Id: seeded.Id}))
	require.NoError(t, err)
	require.Equal(t, seeded.Id, resp.Msg.Manufacturer.Id)
	require.Equal(t, "PT Sido Muncul", resp.Msg.Manufacturer.Name)
}

func TestGetManufacturer_UnknownIsNotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)

	_, err := svc.GetManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.GetManufacturerRequest{Id: "00000000-0000-4000-8000-000000000000"}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetManufacturer_EmptyIdIsInvalidArgument(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)

	_, err := svc.GetManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.GetManufacturerRequest{Id: ""}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

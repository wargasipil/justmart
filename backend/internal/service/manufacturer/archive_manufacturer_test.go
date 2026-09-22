package manufacturer_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestArchiveManufacturer_HidesItFromTheActiveList(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seeded := seedManufacturer(t, svc, "A-1", "Going Away")

	resp, err := svc.ArchiveManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.ArchiveManufacturerRequest{Id: seeded.Id}))
	require.NoError(t, err)
	require.False(t, resp.Msg.Manufacturer.Active)

	list, err := svc.ListManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.ListManufacturersRequest{}))
	require.NoError(t, err)
	require.EqualValues(t, 0, list.Msg.Total)
}

// An archived pabrik must not be offerable in the picker — you cannot assign
// one to a product.
func TestArchiveManufacturer_DropsItFromSearch(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seeded := seedManufacturer(t, svc, "A-2", "Hidden Co")
	_, err := svc.ArchiveManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.ArchiveManufacturerRequest{Id: seeded.Id}))
	require.NoError(t, err)

	found, err := svc.SearchManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.SearchManufacturersRequest{Query: "Hidden"}))
	require.NoError(t, err)
	require.Empty(t, found.Msg.Manufacturers)
}

func TestArchiveManufacturer_UnknownIsNotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)

	_, err := svc.ArchiveManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.ArchiveManufacturerRequest{Id: "00000000-0000-4000-8000-000000000000"}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

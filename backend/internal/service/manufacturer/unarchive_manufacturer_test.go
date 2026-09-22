package manufacturer_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestUnarchiveManufacturer_RestoresIt(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seeded := seedManufacturer(t, svc, "R-1", "Back Again")
	_, err := svc.ArchiveManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.ArchiveManufacturerRequest{Id: seeded.Id}))
	require.NoError(t, err)

	resp, err := svc.UnarchiveManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.UnarchiveManufacturerRequest{Id: seeded.Id}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Manufacturer.Active)

	list, err := svc.ListManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.ListManufacturersRequest{}))
	require.NoError(t, err)
	require.EqualValues(t, 1, list.Msg.Total)
}

// The subtle one: the name index only covers ACTIVE rows, so while this row was
// archived another manufacturer could take its name. Restoring then has to fail
// with a field token rather than a raw unique-index error.
func TestUnarchiveManufacturer_NameTakenWhileArchived(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	original := seedManufacturer(t, svc, "OLD", "Contested Name")
	_, err := svc.ArchiveManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.ArchiveManufacturerRequest{Id: original.Id}))
	require.NoError(t, err)
	seedManufacturer(t, svc, "NEW", "Contested Name")

	_, err = svc.UnarchiveManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.UnarchiveManufacturerRequest{Id: original.Id}))
	requireToken(t, err, connect.CodeAlreadyExists, "manufacturer.name_taken")
}

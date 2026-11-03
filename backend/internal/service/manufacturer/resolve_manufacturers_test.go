package manufacturer_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestResolveManufacturers_ReturnsRefsForKnownIds(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	a := seedManufacturer(t, svc, "RS-1", "Alpha")
	b := seedManufacturer(t, svc, "RS-2", "Beta")

	resp, err := svc.ResolveManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.ResolveManufacturersRequest{Ids: []string{a.Id, b.Id}}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Manufacturers, 2)
	byID := map[string]*inventoryifacev1.ManufacturerRef{}
	for _, r := range resp.Msg.Manufacturers {
		byID[r.Id] = r
	}
	require.Equal(t, "Alpha", byID[a.Id].Name)
	require.Equal(t, "RS-2", byID[b.Id].Code)
}

func TestResolveManufacturers_EmptyInputIsEmptyOutput(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)

	resp, err := svc.ResolveManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.ResolveManufacturersRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Manufacturers)
}

func TestResolveManufacturers_UnknownIdsAreOmittedNotAnError(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	a := seedManufacturer(t, svc, "RS-3", "Known")

	resp, err := svc.ResolveManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.ResolveManufacturersRequest{
			Ids: []string{a.Id, "00000000-0000-4000-8000-000000000000"},
		}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Manufacturers, 1)
	require.Equal(t, a.Id, resp.Msg.Manufacturers[0].Id)
}

// A product can point at an ARCHIVED pabrik, and the detail page still has to
// print its name — so resolve must not filter on active, unlike Search.
func TestResolveManufacturers_IncludesArchived(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	gone := seedManufacturer(t, svc, "RS-4", "Archived Co")
	_, err := svc.ArchiveManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.ArchiveManufacturerRequest{Id: gone.Id}))
	require.NoError(t, err)

	resp, err := svc.ResolveManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.ResolveManufacturersRequest{Ids: []string{gone.Id}}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Manufacturers, 1)
	require.Equal(t, "Archived Co", resp.Msg.Manufacturers[0].Name)
}

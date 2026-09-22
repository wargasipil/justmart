package manufacturer_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestListManufacturers_ActiveOnlyByDefault(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seedManufacturer(t, svc, "L-1", "Alpha")
	archived := seedManufacturer(t, svc, "L-2", "Beta")
	_, err := svc.ArchiveManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.ArchiveManufacturerRequest{Id: archived.Id}))
	require.NoError(t, err)

	resp, err := svc.ListManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.ListManufacturersRequest{}))
	require.NoError(t, err)
	require.EqualValues(t, 1, resp.Msg.Total)
	require.Len(t, resp.Msg.Manufacturers, 1)
	require.Equal(t, "Alpha", resp.Msg.Manufacturers[0].Name)

	withArchived, err := svc.ListManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.ListManufacturersRequest{IncludeInactive: true}))
	require.NoError(t, err)
	require.EqualValues(t, 2, withArchived.Msg.Total)
}

func TestListManufacturers_SearchMatchesNameOrCode(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seedManufacturer(t, svc, "KLB-1", "PT Kalbe Farma")
	seedManufacturer(t, svc, "DXA-1", "PT Dexa Medica")

	byName, err := svc.ListManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.ListManufacturersRequest{Query: "kalbe"}))
	require.NoError(t, err)
	require.EqualValues(t, 1, byName.Msg.Total)

	byCode, err := svc.ListManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.ListManufacturersRequest{Query: "DXA"}))
	require.NoError(t, err)
	require.EqualValues(t, 1, byCode.Msg.Total)
	require.Equal(t, "PT Dexa Medica", byCode.Msg.Manufacturers[0].Name)
}

// total must be the whole matching set, not the size of the page — that is what
// makes "Showing 1–2 of 3" correct.
func TestListManufacturers_PagesWithFullTotal(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seedManufacturer(t, svc, "P-1", "One")
	seedManufacturer(t, svc, "P-2", "Three")
	seedManufacturer(t, svc, "P-3", "Two")

	page, err := svc.ListManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.ListManufacturersRequest{Limit: 2, Offset: 0}))
	require.NoError(t, err)
	require.EqualValues(t, 3, page.Msg.Total)
	require.Len(t, page.Msg.Manufacturers, 2)
	// ORDER BY name: One, Three, Two.
	require.Equal(t, "One", page.Msg.Manufacturers[0].Name)
	require.Equal(t, "Three", page.Msg.Manufacturers[1].Name)

	second, err := svc.ListManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.ListManufacturersRequest{Limit: 2, Offset: 2}))
	require.NoError(t, err)
	require.Len(t, second.Msg.Manufacturers, 1)
	require.Equal(t, "Two", second.Msg.Manufacturers[0].Name)
}

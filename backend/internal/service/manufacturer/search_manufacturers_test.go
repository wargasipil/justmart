package manufacturer_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestSearchManufacturers_MatchesNameOrCode(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seedManufacturer(t, svc, "KLB-1", "PT Kalbe Farma")
	seedManufacturer(t, svc, "DXA-1", "PT Dexa Medica")

	byName, err := svc.SearchManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.SearchManufacturersRequest{Query: "kalbe"}))
	require.NoError(t, err)
	require.Len(t, byName.Msg.Manufacturers, 1)

	byCode, err := svc.SearchManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.SearchManufacturersRequest{Query: "dxa"}))
	require.NoError(t, err)
	require.Len(t, byCode.Msg.Manufacturers, 1)
	require.Equal(t, "PT Dexa Medica", byCode.Msg.Manufacturers[0].Name)
}

func TestSearchManufacturers_EmptyQueryReturnsTopMatches(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seedManufacturer(t, svc, "S-1", "Alpha")
	seedManufacturer(t, svc, "S-2", "Beta")

	resp, err := svc.SearchManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.SearchManufacturersRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Manufacturers, 2)
}

func TestSearchManufacturers_LimitIsClamped(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seedManufacturer(t, svc, "C-1", "One")
	seedManufacturer(t, svc, "C-2", "Two")

	resp, err := svc.SearchManufacturers(context.Background(),
		connect.NewRequest(&inventoryifacev1.SearchManufacturersRequest{Limit: 1}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Manufacturers, 1)
}

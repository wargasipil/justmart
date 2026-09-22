package manufacturer_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestCreateManufacturer_RoundTrip(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)

	resp, err := svc.CreateManufacturer(context.Background(), connect.NewRequest(&inventoryifacev1.CreateManufacturerRequest{
		Code:         "mfr-001", // lowercase in -> uppercased by the handler
		Name:         "  PT Kalbe Farma Tbk  ",
		Address:      "Jl. Letjen Suprapto Kav. 4",
		Phone:        "021-4287-3888",
		ContactEmail: "care@kalbe.co.id",
		Note:         "Prinsipal",
	}))
	require.NoError(t, err)
	m := resp.Msg.Manufacturer
	require.NotEmpty(t, m.Id)
	require.Equal(t, "MFR-001", m.Code)
	require.Equal(t, "PT Kalbe Farma Tbk", m.Name) // trimmed
	require.Equal(t, "Jl. Letjen Suprapto Kav. 4", m.Address)
	require.Equal(t, "021-4287-3888", m.Phone)
	require.Equal(t, "care@kalbe.co.id", m.ContactEmail)
	require.Equal(t, "Prinsipal", m.Note)
	require.True(t, m.Active)
}

func TestCreateManufacturer_MissingCodeOrName(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)

	_, err := svc.CreateManufacturer(context.Background(), connect.NewRequest(&inventoryifacev1.CreateManufacturerRequest{
		Code: "MFR-002",
		Name: "   ", // trims to empty
	}))
	requireToken(t, err, connect.CodeInvalidArgument, "manufacturer.required")
}

func TestCreateManufacturer_DuplicateCode(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seedManufacturer(t, svc, "DUP-1", "First")

	_, err := svc.CreateManufacturer(context.Background(), connect.NewRequest(&inventoryifacev1.CreateManufacturerRequest{
		Code: "DUP-1",
		Name: "Second",
	}))
	requireToken(t, err, connect.CodeAlreadyExists, "manufacturer.code_taken")
}

func TestCreateManufacturer_DuplicateActiveName(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seedManufacturer(t, svc, "CODE-A", "Same Name")

	_, err := svc.CreateManufacturer(context.Background(), connect.NewRequest(&inventoryifacev1.CreateManufacturerRequest{
		Code: "CODE-B",
		Name: "Same Name",
	}))
	requireToken(t, err, connect.CodeAlreadyExists, "manufacturer.name_taken")
}

// The name index covers ACTIVE rows only, so archiving frees the name. This is
// the difference between the code rule and the name rule, and it is easy to
// break by "simplifying" the index.
func TestCreateManufacturer_ArchivedNameIsFree(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	first := seedManufacturer(t, svc, "OLD-1", "Reused Name")
	_, err := svc.ArchiveManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.ArchiveManufacturerRequest{Id: first.Id}))
	require.NoError(t, err)

	_, err = svc.CreateManufacturer(context.Background(), connect.NewRequest(&inventoryifacev1.CreateManufacturerRequest{
		Code: "NEW-1",
		Name: "Reused Name",
	}))
	require.NoError(t, err)
}

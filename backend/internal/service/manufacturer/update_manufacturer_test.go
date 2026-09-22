package manufacturer_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestUpdateManufacturer_RoundTrip(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seeded := seedManufacturer(t, svc, "U-1", "Before")

	resp, err := svc.UpdateManufacturer(context.Background(), connect.NewRequest(&inventoryifacev1.UpdateManufacturerRequest{
		Id:           seeded.Id,
		Code:         "u-2",
		Name:         "After",
		Address:      "Bandung",
		Phone:        "022-1",
		ContactEmail: "a@b.test",
		Note:         "catatan",
	}))
	require.NoError(t, err)
	m := resp.Msg.Manufacturer
	require.Equal(t, "U-2", m.Code)
	require.Equal(t, "After", m.Name)
	require.Equal(t, "Bandung", m.Address)
	require.Equal(t, "catatan", m.Note)

	// Persisted, not just echoed.
	got, err := svc.GetManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.GetManufacturerRequest{Id: seeded.Id}))
	require.NoError(t, err)
	require.Equal(t, "After", got.Msg.Manufacturer.Name)
	require.Equal(t, "U-2", got.Msg.Manufacturer.Code)
}

// Re-saving a row unchanged must not collide with itself — that is what the
// excludeID arm of assertUnique is for.
func TestUpdateManufacturer_SelfIsNotACollision(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seeded := seedManufacturer(t, svc, "U-SELF", "Unchanged")

	_, err := svc.UpdateManufacturer(context.Background(), connect.NewRequest(&inventoryifacev1.UpdateManufacturerRequest{
		Id:   seeded.Id,
		Code: seeded.Code,
		Name: seeded.Name,
	}))
	require.NoError(t, err)
}

func TestUpdateManufacturer_DuplicateCode(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seedManufacturer(t, svc, "TAKEN", "Other")
	mine := seedManufacturer(t, svc, "MINE", "Mine")

	_, err := svc.UpdateManufacturer(context.Background(), connect.NewRequest(&inventoryifacev1.UpdateManufacturerRequest{
		Id:   mine.Id,
		Code: "TAKEN",
		Name: "Mine",
	}))
	requireToken(t, err, connect.CodeAlreadyExists, "manufacturer.code_taken")
}

func TestUpdateManufacturer_RequiresCodeAndName(t *testing.T) {
	t.Parallel()
	svc, _ := newEnv(t)
	seeded := seedManufacturer(t, svc, "U-REQ", "Named")

	_, err := svc.UpdateManufacturer(context.Background(), connect.NewRequest(&inventoryifacev1.UpdateManufacturerRequest{
		Id:   seeded.Id,
		Code: "U-REQ",
		Name: "  ",
	}))
	requireToken(t, err, connect.CodeInvalidArgument, "manufacturer.required")
}

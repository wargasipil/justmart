package manufacturer_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	mfrsvc "github.com/justmart/backend/internal/service/manufacturer"
	"github.com/justmart/backend/internal/service/servicetest"
)

// Shared fixtures for the manufacturer suite. Engine-agnostic: everything goes
// through servicetest, so the same files run on SQLite (`make test-unit`) and
// on Postgres (`make test-unit-postgres`).

func newEnv(t *testing.T) (*mfrsvc.ManufacturerService, *gorm.DB) {
	t.Helper()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	return mfrsvc.NewManufacturerService(db), db
}

// requireToken asserts a connect error carries the given code + stable token
// message (the contract frontend serverErrors.ts relies on).
func requireToken(t *testing.T, err error, code connect.Code, token string) {
	t.Helper()
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, code, ce.Code())
	require.Equal(t, token, ce.Message())
}

func seedManufacturer(t *testing.T, svc *mfrsvc.ManufacturerService, code, name string) *inventoryifacev1.Manufacturer {
	t.Helper()
	resp, err := svc.CreateManufacturer(context.Background(),
		connect.NewRequest(&inventoryifacev1.CreateManufacturerRequest{Code: code, Name: name}))
	require.NoError(t, err)
	return resp.Msg.Manufacturer
}

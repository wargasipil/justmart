package settings_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
	settingssvc "github.com/justmart/backend/internal/service/settings"
	"github.com/justmart/backend/internal/service/servicetest"
)

// SetBussinessSettings persists the type; GetBussinessSettings reads it back; a
// second Set overwrites (upsert).
func TestSetBussinessSettings_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	ctx := context.Background()

	setResp, err := svc.SetBussinessSettings(ctx, connect.NewRequest(&settingsifacev1.SetBussinessSettingsRequest{
		Type: settingsifacev1.BussinessType_BUSSINESS_TYPE_RETAIL,
	}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_RETAIL, setResp.Msg.Type)

	getResp, err := svc.GetBussinessSettings(ctx, connect.NewRequest(&settingsifacev1.GetBussinessSettingsRequest{}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_RETAIL, getResp.Msg.Type)

	// Upsert: a second Set replaces the stored value.
	_, err = svc.SetBussinessSettings(ctx, connect.NewRequest(&settingsifacev1.SetBussinessSettingsRequest{
		Type: settingsifacev1.BussinessType_BUSSINESS_TYPE_PHARMACY_SHOP,
	}))
	require.NoError(t, err)

	getResp2, err := svc.GetBussinessSettings(ctx, connect.NewRequest(&settingsifacev1.GetBussinessSettingsRequest{}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_PHARMACY_SHOP, getResp2.Msg.Type)
}

// UNSPECIFIED is rejected — a business type must be concrete.
func TestSetBussinessSettings_RejectsUnspecified(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.SetBussinessSettings(context.Background(), connect.NewRequest(&settingsifacev1.SetBussinessSettingsRequest{
		Type: settingsifacev1.BussinessType_BUSSINESS_TYPE_UNSPECIFIED,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// When a license is applied it owns the mode — the manual setter is rejected so
// the mode can't drift from (and silently revert to) the licensed type.
func TestSetBussinessSettings_RejectedWhenLicensed(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := settingssvc.NewSettingsService(db)
	ctx := context.Background()
	require.NoError(t, common.SetLicense(ctx, db, "stored-license-token", "Apotek X"))

	_, err := svc.SetBussinessSettings(ctx, connect.NewRequest(&settingsifacev1.SetBussinessSettingsRequest{
		Type: settingsifacev1.BussinessType_BUSSINESS_TYPE_RETAIL,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

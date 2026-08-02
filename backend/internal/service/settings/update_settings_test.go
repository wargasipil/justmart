package settings_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	settingssvc "github.com/justmart/backend/internal/service/settings"
	"github.com/justmart/backend/internal/service/servicetest"
)

// UpdateSettings upserts the threshold and echoes it back in the response. A
// second call updates the same key (OnConflict DO UPDATE), so the latest value
// wins — verified by re-reading via GetSettings.
func TestUpdateSettings_Upsert(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.UpdateSettings(context.Background(), connect.NewRequest(&settingsifacev1.UpdateSettingsRequest{
		LowStockThreshold: 5,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Settings)
	require.Equal(t, int32(5), resp.Msg.Settings.LowStockThreshold)

	// Update the same key again — the conflict clause overwrites it.
	resp2, err := svc.UpdateSettings(context.Background(), connect.NewRequest(&settingsifacev1.UpdateSettingsRequest{
		LowStockThreshold: 25,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(25), resp2.Msg.Settings.LowStockThreshold)

	got, err := svc.GetSettings(context.Background(), connect.NewRequest(&settingsifacev1.GetSettingsRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(25), got.Msg.Settings.LowStockThreshold)
}

// A zero threshold is valid (>= 0) and persists.
func TestUpdateSettings_ZeroAllowed(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.UpdateSettings(context.Background(), connect.NewRequest(&settingsifacev1.UpdateSettingsRequest{
		LowStockThreshold: 0,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(0), resp.Msg.Settings.LowStockThreshold)
}

// A negative threshold is rejected with InvalidArgument before any DB write.
func TestUpdateSettings_NegativeRejected(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UpdateSettings(context.Background(), connect.NewRequest(&settingsifacev1.UpdateSettingsRequest{
		LowStockThreshold: -1,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// The app title is trimmed, persisted, and readable back — it brands the tab
// title / sidebar / login screen. Passing "" clears the override.
func TestUpdateSettings_AppTitlePersistsAndClears(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	ctx := context.Background()

	resp, err := svc.UpdateSettings(ctx, connect.NewRequest(&settingsifacev1.UpdateSettingsRequest{
		LowStockThreshold: 10,
		AppTitle:          "  Toko Maju  ",
	}))
	require.NoError(t, err)
	require.Equal(t, "Toko Maju", resp.Msg.Settings.AppTitle)

	brand, err := svc.GetBranding(ctx, connect.NewRequest(&settingsifacev1.GetBrandingRequest{}))
	require.NoError(t, err)
	require.Equal(t, "Toko Maju", brand.Msg.AppTitle)

	// Empty clears it — the app falls back to the built-in brand.
	_, err = svc.UpdateSettings(ctx, connect.NewRequest(&settingsifacev1.UpdateSettingsRequest{
		LowStockThreshold: 10,
		AppTitle:          "",
	}))
	require.NoError(t, err)
	got, err := svc.GetSettings(ctx, connect.NewRequest(&settingsifacev1.GetSettingsRequest{}))
	require.NoError(t, err)
	require.Empty(t, got.Msg.Settings.AppTitle)
}

// An over-long title is rejected with a stable token, nothing is written.
func TestUpdateSettings_AppTitleTooLong(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	ctx := context.Background()

	_, err := svc.UpdateSettings(ctx, connect.NewRequest(&settingsifacev1.UpdateSettingsRequest{
		LowStockThreshold: 10,
		AppTitle:          strings.Repeat("x", settingssvc.MaxAppTitleLen+1),
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	require.Equal(t, "settings.app_title_too_long", ce.Message())

	got, err := svc.GetSettings(ctx, connect.NewRequest(&settingsifacev1.GetSettingsRequest{}))
	require.NoError(t, err)
	require.Empty(t, got.Msg.Settings.AppTitle)
}

// The business mode is settable from Settings ▸ General; an UNSPECIFIED value on
// a later call means "leave it alone" so an unrelated edit can't reset the mode.
func TestUpdateSettings_BusinessMode(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	ctx := context.Background()

	resp, err := svc.UpdateSettings(ctx, connect.NewRequest(&settingsifacev1.UpdateSettingsRequest{
		LowStockThreshold: 10,
		BusinessType:      settingsifacev1.BussinessType_BUSSINESS_TYPE_PHARMACY_SHOP,
	}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_PHARMACY_SHOP, resp.Msg.Settings.BusinessType)

	mode, err := svc.GetBussinessSettings(ctx, connect.NewRequest(&settingsifacev1.GetBussinessSettingsRequest{}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_PHARMACY_SHOP, mode.Msg.Type)

	// Threshold-only edit (UNSPECIFIED mode) keeps the pharmacy mode.
	resp2, err := svc.UpdateSettings(ctx, connect.NewRequest(&settingsifacev1.UpdateSettingsRequest{
		LowStockThreshold: 7,
	}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_PHARMACY_SHOP, resp2.Msg.Settings.BusinessType)

	// Switching back to retail persists.
	_, err = svc.UpdateSettings(ctx, connect.NewRequest(&settingsifacev1.UpdateSettingsRequest{
		LowStockThreshold: 7,
		BusinessType:      settingsifacev1.BussinessType_BUSSINESS_TYPE_RETAIL,
	}))
	require.NoError(t, err)
	mode2, err := svc.GetBussinessSettings(ctx, connect.NewRequest(&settingsifacev1.GetBussinessSettingsRequest{}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_RETAIL, mode2.Msg.Type)
}

// Restaurant is a first-class third mode: settable, persisted, and readable back
// through BOTH the authenticated business-settings RPC and the public branding
// RPC (the login screen brands off the latter, so a restaurant must not fall
// back to the retail brand pre-login).
func TestUpdateSettings_RestaurantMode(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	ctx := context.Background()

	resp, err := svc.UpdateSettings(ctx, connect.NewRequest(&settingsifacev1.UpdateSettingsRequest{
		LowStockThreshold: 10,
		AppTitle:          "Warung Sederhana",
		BusinessType:      settingsifacev1.BussinessType_BUSSINESS_TYPE_RESTAURANT,
	}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_RESTAURANT, resp.Msg.Settings.BusinessType)

	mode, err := svc.GetBussinessSettings(ctx, connect.NewRequest(&settingsifacev1.GetBussinessSettingsRequest{}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_RESTAURANT, mode.Msg.Type)

	brand, err := svc.GetBranding(ctx, connect.NewRequest(&settingsifacev1.GetBrandingRequest{}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_RESTAURANT, brand.Msg.BusinessType)
	require.Equal(t, "Warung Sederhana", brand.Msg.AppTitle)

	// A threshold-only edit must not silently reset a restaurant to retail —
	// same UNSPECIFIED-preserves rule the pharmacy mode relies on.
	resp2, err := svc.UpdateSettings(ctx, connect.NewRequest(&settingsifacev1.UpdateSettingsRequest{
		LowStockThreshold: 3,
		AppTitle:          "Warung Sederhana",
	}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_RESTAURANT, resp2.Msg.Settings.BusinessType)
}

// An unknown business type value is rejected with a stable token.
func TestUpdateSettings_UnknownBusinessType(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UpdateSettings(context.Background(), connect.NewRequest(&settingsifacev1.UpdateSettingsRequest{
		LowStockThreshold: 10,
		BusinessType:      settingsifacev1.BussinessType(99),
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	require.Equal(t, "settings.business_type_invalid", ce.Message())
}

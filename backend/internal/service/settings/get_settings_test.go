package settings_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	settingssvc "github.com/justmart/backend/internal/service/settings"
	"github.com/justmart/backend/internal/service/servicetest"
)

// With no app_settings row present, GetSettings returns the default low-stock
// threshold (10) via common.GetLowStockThreshold's record-not-found fallback.
func TestGetSettings_DefaultWhenUnset(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.GetSettings(context.Background(), connect.NewRequest(&settingsifacev1.GetSettingsRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Settings)
	require.Equal(t, int32(10), resp.Msg.Settings.LowStockThreshold)
}

// After UpdateSettings writes a value, GetSettings reads it back.
func TestGetSettings_ReflectsStoredValue(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UpdateSettings(context.Background(), connect.NewRequest(&settingsifacev1.UpdateSettingsRequest{
		LowStockThreshold: 42,
	}))
	require.NoError(t, err)

	resp, err := svc.GetSettings(context.Background(), connect.NewRequest(&settingsifacev1.GetSettingsRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Settings)
	require.Equal(t, int32(42), resp.Msg.Settings.LowStockThreshold)
}

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

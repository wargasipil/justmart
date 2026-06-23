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

func TestFeatureFlags_DefaultOffThenToggle(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := settingssvc.NewSettingsService(db)
	ctx := context.Background()

	// Default: payroll feature off (no row yet).
	got, err := svc.GetFeatureFlags(ctx, connect.NewRequest(&settingsifacev1.GetFeatureFlagsRequest{}))
	require.NoError(t, err)
	require.False(t, got.Msg.Flags.PayrollEnabled)

	// Enable it.
	set, err := svc.SetFeatureFlags(ctx, connect.NewRequest(&settingsifacev1.SetFeatureFlagsRequest{PayrollEnabled: true}))
	require.NoError(t, err)
	require.True(t, set.Msg.Flags.PayrollEnabled)

	// Persisted.
	got, err = svc.GetFeatureFlags(ctx, connect.NewRequest(&settingsifacev1.GetFeatureFlagsRequest{}))
	require.NoError(t, err)
	require.True(t, got.Msg.Flags.PayrollEnabled)

	// Disable it again.
	_, err = svc.SetFeatureFlags(ctx, connect.NewRequest(&settingsifacev1.SetFeatureFlagsRequest{PayrollEnabled: false}))
	require.NoError(t, err)
	got, err = svc.GetFeatureFlags(ctx, connect.NewRequest(&settingsifacev1.GetFeatureFlagsRequest{}))
	require.NoError(t, err)
	require.False(t, got.Msg.Flags.PayrollEnabled)
}

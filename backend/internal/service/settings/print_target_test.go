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

// GetPrintTarget returns empty strings when no default has been saved.
func TestGetPrintTarget_DefaultWhenUnset(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	resp, err := svc.GetPrintTarget(context.Background(), connect.NewRequest(&settingsifacev1.GetPrintTargetRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.ConnectorDeviceId)
	require.Empty(t, resp.Msg.PrinterName)
}

// SetPrintTarget persists (trimming whitespace) + GetPrintTarget reads it back.
func TestSetPrintTarget_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	ctx := context.Background()

	set, err := svc.SetPrintTarget(ctx, connect.NewRequest(&settingsifacev1.SetPrintTargetRequest{
		ConnectorDeviceId: "  dev-1  ", PrinterName: "POS-58",
	}))
	require.NoError(t, err)
	require.Equal(t, "dev-1", set.Msg.ConnectorDeviceId)
	require.Equal(t, "POS-58", set.Msg.PrinterName)

	got, err := svc.GetPrintTarget(ctx, connect.NewRequest(&settingsifacev1.GetPrintTargetRequest{}))
	require.NoError(t, err)
	require.Equal(t, "dev-1", got.Msg.ConnectorDeviceId)
	require.Equal(t, "POS-58", got.Msg.PrinterName)
}

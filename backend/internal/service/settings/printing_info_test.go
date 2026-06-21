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

// GetPrintingInfo defaults to "tcp" when no mode was set, and reports no local
// printers (usb-only).
func TestGetPrintingInfo_DefaultsToTcp(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	resp, err := svc.GetPrintingInfo(context.Background(), connect.NewRequest(&settingsifacev1.GetPrintingInfoRequest{}))
	require.NoError(t, err)
	require.Equal(t, "tcp", resp.Msg.Mode)
	require.Empty(t, resp.Msg.LocalPrinters) // not usb mode → no enumeration
}

// The configured mode (SetConnectorMode) is reported back.
func TestGetPrintingInfo_ReportsConfiguredMode(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	svc.SetConnectorMode("connector")
	resp, err := svc.GetPrintingInfo(context.Background(), connect.NewRequest(&settingsifacev1.GetPrintingInfoRequest{}))
	require.NoError(t, err)
	require.Equal(t, "connector", resp.Msg.Mode)
}

// usb mode reports the mode; local printers come from the OS spooler (empty on
// the non-Windows test host / CI — asserting it returns cleanly is enough).
func TestGetPrintingInfo_UsbMode(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	svc.SetConnectorMode("usb")
	resp, err := svc.GetPrintingInfo(context.Background(), connect.NewRequest(&settingsifacev1.GetPrintingInfoRequest{}))
	require.NoError(t, err)
	require.Equal(t, "usb", resp.Msg.Mode)
}

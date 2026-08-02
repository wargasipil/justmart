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

// Unset means "not configured" — both fields empty. The fire path reads that as
// "fall back to the receipt printer", which is what makes a one-printer warung
// work with no setup.
func TestGetKitchenPrintTarget_DefaultsEmpty(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	got, err := svc.GetKitchenPrintTarget(context.Background(),
		connect.NewRequest(&settingsifacev1.GetKitchenPrintTargetRequest{}))
	require.NoError(t, err)
	require.Empty(t, got.Msg.ConnectorDeviceId)
	require.Empty(t, got.Msg.PrinterName)
}

// The kitchen target persists and is INDEPENDENT of the receipt target — a shop
// with a pass printer must be able to point the two documents at different
// hardware.
func TestSetKitchenPrintTarget_PersistsIndependentlyOfReceipt(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	ctx := context.Background()

	_, err := svc.SetPrintTarget(ctx, connect.NewRequest(&settingsifacev1.SetPrintTargetRequest{
		ConnectorDeviceId: "dev-till", PrinterName: "TILL-58",
	}))
	require.NoError(t, err)
	_, err = svc.SetKitchenPrintTarget(ctx, connect.NewRequest(&settingsifacev1.SetKitchenPrintTargetRequest{
		ConnectorDeviceId: "  dev-pass  ", PrinterName: "  PASS-80  ",
	}))
	require.NoError(t, err)

	kitchen, err := svc.GetKitchenPrintTarget(ctx,
		connect.NewRequest(&settingsifacev1.GetKitchenPrintTargetRequest{}))
	require.NoError(t, err)
	require.Equal(t, "dev-pass", kitchen.Msg.ConnectorDeviceId)
	require.Equal(t, "PASS-80", kitchen.Msg.PrinterName)

	receipt, err := svc.GetPrintTarget(ctx, connect.NewRequest(&settingsifacev1.GetPrintTargetRequest{}))
	require.NoError(t, err)
	require.Equal(t, "dev-till", receipt.Msg.ConnectorDeviceId,
		"setting the kitchen target must not disturb the receipt target")
}

// Clearing returns the shop to the receipt printer.
func TestSetKitchenPrintTarget_EmptyClearsOverride(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	ctx := context.Background()

	_, err := svc.SetKitchenPrintTarget(ctx, connect.NewRequest(&settingsifacev1.SetKitchenPrintTargetRequest{
		ConnectorDeviceId: "dev-pass", PrinterName: "PASS-80",
	}))
	require.NoError(t, err)
	_, err = svc.SetKitchenPrintTarget(ctx, connect.NewRequest(&settingsifacev1.SetKitchenPrintTargetRequest{}))
	require.NoError(t, err)

	got, err := svc.GetKitchenPrintTarget(ctx,
		connect.NewRequest(&settingsifacev1.GetKitchenPrintTargetRequest{}))
	require.NoError(t, err)
	require.Empty(t, got.Msg.ConnectorDeviceId)
	require.Empty(t, got.Msg.PrinterName)
}

package settings

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// GetKitchenPrintTarget returns the saved KITCHEN ticket target — where
// SaleService.FireToKitchen sends a ticket when the request carries none.
//
// It is a separate setting from the receipt target because the whole point of a
// kitchen ticket is that it comes out at the pass rather than at the till. Both
// values empty means "not configured", and the fire path then falls back to the
// receipt target — so a one-printer warung works with no setup at all.
func (s *SettingsService) GetKitchenPrintTarget(
	ctx context.Context,
	_ *connect.Request[settingsifacev1.GetKitchenPrintTargetRequest],
) (*connect.Response[settingsifacev1.GetKitchenPrintTargetResponse], error) {
	deviceID, printerName, err := common.GetKitchenPrintTarget(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.GetKitchenPrintTargetResponse{
		ConnectorDeviceId: deviceID,
		PrinterName:       printerName,
	}), nil
}

// SetKitchenPrintTarget persists the kitchen ticket target (owner-only). Setting
// both fields empty clears the override and returns the shop to the receipt
// printer.
func (s *SettingsService) SetKitchenPrintTarget(
	ctx context.Context,
	req *connect.Request[settingsifacev1.SetKitchenPrintTargetRequest],
) (*connect.Response[settingsifacev1.SetKitchenPrintTargetResponse], error) {
	deviceID := strings.TrimSpace(req.Msg.ConnectorDeviceId)
	printerName := strings.TrimSpace(req.Msg.PrinterName)
	if err := common.SetKitchenPrintTarget(ctx, s.db, deviceID, printerName); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.SetKitchenPrintTargetResponse{
		ConnectorDeviceId: deviceID,
		PrinterName:       printerName,
	}), nil
}

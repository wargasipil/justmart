package settings

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// GetPrintTarget returns the saved default print connector + printer (the
// target SaleService.PrintReceipt uses when the request carries none). Backs
// the Settings ▸ Printing panel.
func (s *SettingsService) GetPrintTarget(
	ctx context.Context,
	_ *connect.Request[settingsifacev1.GetPrintTargetRequest],
) (*connect.Response[settingsifacev1.GetPrintTargetResponse], error) {
	deviceID, printerName, err := common.GetPrintTarget(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.GetPrintTargetResponse{
		ConnectorDeviceId: deviceID,
		PrinterName:       printerName,
	}), nil
}

// SetPrintTarget persists the default print connector + printer (owner-only).
func (s *SettingsService) SetPrintTarget(
	ctx context.Context,
	req *connect.Request[settingsifacev1.SetPrintTargetRequest],
) (*connect.Response[settingsifacev1.SetPrintTargetResponse], error) {
	deviceID := strings.TrimSpace(req.Msg.ConnectorDeviceId)
	printerName := strings.TrimSpace(req.Msg.PrinterName)
	if err := common.SetPrintTarget(ctx, s.db, deviceID, printerName); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.SetPrintTargetResponse{
		ConnectorDeviceId: deviceID,
		PrinterName:       printerName,
	}), nil
}

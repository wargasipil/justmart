package settings

import (
	"context"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// GetReceiptSettings returns the printed-receipt header + footer (multi-line
// strings). Seeded at boot from config.yaml, so a booted server returns the
// effective values; editable here in Settings ▸ Printing.
func (s *SettingsService) GetReceiptSettings(
	ctx context.Context,
	_ *connect.Request[settingsifacev1.GetReceiptSettingsRequest],
) (*connect.Response[settingsifacev1.GetReceiptSettingsResponse], error) {
	header, footer, err := common.GetReceiptText(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.GetReceiptSettingsResponse{
		Header: header,
		Footer: footer,
	}), nil
}

// SetReceiptSettings persists the receipt header + footer (owner-only).
func (s *SettingsService) SetReceiptSettings(
	ctx context.Context,
	req *connect.Request[settingsifacev1.SetReceiptSettingsRequest],
) (*connect.Response[settingsifacev1.SetReceiptSettingsResponse], error) {
	if err := common.SetReceiptText(ctx, s.db, req.Msg.Header, req.Msg.Footer); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.SetReceiptSettingsResponse{
		Header: req.Msg.Header,
		Footer: req.Msg.Footer,
	}), nil
}

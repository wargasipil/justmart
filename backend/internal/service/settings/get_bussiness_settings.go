package settings

import (
	"context"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// GetBussinessSettings returns the shop's configured business type (UNSPECIFIED
// when it has never been set) plus the configured app title. Readable by every
// authenticated role — the mode + title drive branding, navigation and POS
// behavior for all users.
func (s *SettingsService) GetBussinessSettings(
	ctx context.Context,
	_ *connect.Request[settingsifacev1.GetBussinessSettingsRequest],
) (*connect.Response[settingsifacev1.GetBussinessSettingsResponse], error) {
	n, err := common.GetBussinessType(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	title, err := common.GetAppTitle(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.GetBussinessSettingsResponse{
		Type:     settingsifacev1.BussinessType(n),
		AppTitle: title,
	}), nil
}

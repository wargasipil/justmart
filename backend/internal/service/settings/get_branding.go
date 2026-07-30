package settings

import (
	"context"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// GetBranding returns the minimal branding facts (business mode + app title) the
// app chrome needs BEFORE the user logs in — the login screen and the browser
// tab title. PUBLIC (see the proto annotation): reads only the storefront brand,
// which is shown to anyone reaching the login page anyway. All richer settings
// RPCs stay role-gated.
func (s *SettingsService) GetBranding(
	ctx context.Context,
	_ *connect.Request[settingsifacev1.GetBrandingRequest],
) (*connect.Response[settingsifacev1.GetBrandingResponse], error) {
	bt, err := common.GetBussinessType(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	title, err := common.GetAppTitle(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.GetBrandingResponse{
		BusinessType: settingsifacev1.BussinessType(bt),
		AppTitle:     title,
	}), nil
}

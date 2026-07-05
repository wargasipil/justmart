package settings

import (
	"context"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// GetBranding returns the minimal branding facts (business type + licensed shop
// name) the app chrome needs BEFORE the user logs in — the login screen and the
// browser tab title. PUBLIC (see the proto annotation): reads only the storefront
// brand, which is shown to anyone reaching the login page anyway. All richer
// settings/license RPCs stay role-gated.
func (s *SettingsService) GetBranding(
	ctx context.Context,
	_ *connect.Request[settingsifacev1.GetBrandingRequest],
) (*connect.Response[settingsifacev1.GetBrandingResponse], error) {
	bt, err := common.GetBussinessType(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	name, err := common.GetLicenseName(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.GetBrandingResponse{
		BusinessType: settingsifacev1.BussinessType(bt),
		ShopName:     name,
	}), nil
}

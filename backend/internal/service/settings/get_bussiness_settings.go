package settings

import (
	"context"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// GetBussinessSettings returns the shop's configured business type (UNSPECIFIED
// when it has never been set).
func (s *SettingsService) GetBussinessSettings(
	ctx context.Context,
	_ *connect.Request[settingsifacev1.GetBussinessSettingsRequest],
) (*connect.Response[settingsifacev1.GetBussinessSettingsResponse], error) {
	n, err := common.GetBussinessType(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// Licensed shop name — surfaced to every role so the app chrome can brand by
	// it (e.g. the pharmacy-mode header). Empty when unlicensed.
	name, err := common.GetLicenseName(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.GetBussinessSettingsResponse{
		Type: settingsifacev1.BussinessType(n),
		Name: name,
	}), nil
}

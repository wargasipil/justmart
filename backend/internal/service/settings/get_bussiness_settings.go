package settings

import (
	"context"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
)

// GetBussinessSettings is a stub for the (in-progress) per-business-type
// settings RPC: it accepts a BussinessType and returns an empty response. Wire
// real business-type-specific settings here when the feature is fleshed out.
func (s *SettingsService) GetBussinessSettings(
	_ context.Context,
	_ *connect.Request[settingsifacev1.GetBussinessSettingsRequest],
) (*connect.Response[settingsifacev1.GetBussinessSettingsResponse], error) {
	return connect.NewResponse(&settingsifacev1.GetBussinessSettingsResponse{}), nil
}

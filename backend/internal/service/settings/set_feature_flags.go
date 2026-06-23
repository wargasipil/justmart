package settings

import (
	"context"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// SetFeatureFlags persists the beta feature toggles. OWNER-only (proto-gated).
func (s *SettingsService) SetFeatureFlags(
	ctx context.Context,
	req *connect.Request[settingsifacev1.SetFeatureFlagsRequest],
) (*connect.Response[settingsifacev1.SetFeatureFlagsResponse], error) {
	if err := common.SetFeaturePayroll(ctx, s.db, req.Msg.PayrollEnabled); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.SetFeatureFlagsResponse{
		Flags: &settingsifacev1.FeatureFlags{PayrollEnabled: req.Msg.PayrollEnabled},
	}), nil
}

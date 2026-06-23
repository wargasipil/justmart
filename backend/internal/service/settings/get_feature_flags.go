package settings

import (
	"context"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// GetFeatureFlags reports the beta feature toggles. Readable by every
// authenticated role so the sidebar can gate menu items uniformly.
func (s *SettingsService) GetFeatureFlags(
	ctx context.Context,
	_ *connect.Request[settingsifacev1.GetFeatureFlagsRequest],
) (*connect.Response[settingsifacev1.GetFeatureFlagsResponse], error) {
	payroll, err := common.GetFeaturePayroll(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.GetFeatureFlagsResponse{
		Flags: &settingsifacev1.FeatureFlags{PayrollEnabled: payroll},
	}), nil
}

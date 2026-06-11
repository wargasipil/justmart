package settings

import (
	"context"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/license"
	"github.com/justmart/backend/internal/service/common"
)

// GetLicenseInfo reports the currently-applied license: whether one is stored,
// the holder name, and the active business type (so the Settings UI can show
// "Licensed to X — Pharmacy mode"). Owner-only.
func (s *SettingsService) GetLicenseInfo(
	ctx context.Context,
	req *connect.Request[settingsifacev1.GetLicenseInfoRequest],
) (*connect.Response[settingsifacev1.GetLicenseInfoResponse], error) {
	token, err := common.GetLicense(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	name, err := common.GetLicenseName(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// The mode shown for a licensed shop is the LICENSE's business type
	// (re-verified from the stored token), not the raw business_type setting —
	// so it can never contradict the applied license. Fall back to the stored
	// mode only when there's no license (or the stored token no longer verifies).
	var bt int32
	if token != "" {
		if claims, verr := license.Verify(token); verr == nil {
			bt = claims.BusinessType
		} else if bt, err = common.GetBussinessType(ctx, s.db); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else if bt, err = common.GetBussinessType(ctx, s.db); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&settingsifacev1.GetLicenseInfoResponse{
		HasLicense: token != "",
		Name:       name,
		Type:       settingsifacev1.BussinessType(bt),
	}), nil
}

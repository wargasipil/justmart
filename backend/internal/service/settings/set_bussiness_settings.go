package settings

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// SetBussinessSettings persists the shop's business type (upsert into the
// app_settings key/value table) and echoes the saved value back. The type must
// be a concrete BussinessType — UNSPECIFIED / unknown values are rejected.
func (s *SettingsService) SetBussinessSettings(
	ctx context.Context,
	req *connect.Request[settingsifacev1.SetBussinessSettingsRequest],
) (*connect.Response[settingsifacev1.SetBussinessSettingsResponse], error) {
	t := req.Msg.Type
	if t == settingsifacev1.BussinessType_BUSSINESS_TYPE_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("business type must be specified"))
	}
	if _, ok := settingsifacev1.BussinessType_name[int32(t)]; !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("unknown business type"))
	}

	// When a license is applied it owns the mode (and re-applies it on every
	// boot), so a manual override here would silently revert and could diverge
	// from the licensed type. Reject it; the owner must change the license. With
	// no license stored this manual setter stays available (unlicensed install).
	if licenseToken, err := common.GetLicense(ctx, s.db); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if licenseToken != "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("business mode is set by the license; apply a license to change it"))
	}

	if err := common.SetBussinessType(ctx, s.db, int32(t)); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.SetBussinessSettingsResponse{Type: t}), nil
}

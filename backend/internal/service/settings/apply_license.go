package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/license"
	"github.com/justmart/backend/internal/service/common"
)

// ApplyLicense verifies a license token entered in the Settings UI, persists it
// (so it survives reboots and is re-applied on boot), and applies its business
// type as the active mode. Owner-only. An invalid token is rejected; nothing is
// persisted on failure.
func (s *SettingsService) ApplyLicense(
	ctx context.Context,
	req *connect.Request[settingsifacev1.ApplyLicenseRequest],
) (*connect.Response[settingsifacev1.ApplyLicenseResponse], error) {
	token := strings.TrimSpace(req.Msg.Token)
	if token == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("license token required"))
	}

	claims, err := license.Verify(token)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid license: %w", err))
	}
	if strings.TrimSpace(claims.Name) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("license has no holder name"))
	}
	bt := claims.BusinessType
	if bt == int32(settingsifacev1.BussinessType_BUSSINESS_TYPE_UNSPECIFIED) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("license has no business type"))
	}
	if _, ok := settingsifacev1.BussinessType_name[bt]; !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("license has an unknown business type"))
	}

	// Persist the token + name and apply the mode atomically.
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := common.SetLicense(ctx, tx, token, claims.Name); err != nil {
			return err
		}
		return common.SetBussinessType(ctx, tx, bt)
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&settingsifacev1.ApplyLicenseResponse{
		Name: claims.Name,
		Type: settingsifacev1.BussinessType(bt),
	}), nil
}

package settings

import (
	"context"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// MaxAppTitleLen bounds the configurable shop/app title — it renders in the
// browser tab, the sidebar brand and the login heading, so an unbounded string
// would just overflow those surfaces.
const MaxAppTitleLen = 60

// UpdateSettings persists the app-wide settings edited in Settings ▸ General
// (owner-only): the low-stock threshold, the shop/app title, and the business
// mode. An empty app_title clears the override (built-in brand); an UNSPECIFIED
// business_type leaves the current mode unchanged.
func (s *SettingsService) UpdateSettings(
	ctx context.Context,
	req *connect.Request[settingsifacev1.UpdateSettingsRequest],
) (*connect.Response[settingsifacev1.UpdateSettingsResponse], error) {
	if req.Msg.LowStockThreshold < 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "settings.threshold_negative")
	}
	title := strings.TrimSpace(req.Msg.AppTitle)
	if len([]rune(title)) > MaxAppTitleLen {
		return nil, common.TokenError(connect.CodeInvalidArgument, "settings.app_title_too_long")
	}
	bt := req.Msg.BusinessType
	if _, ok := settingsifacev1.BussinessType_name[int32(bt)]; !ok {
		return nil, common.TokenError(connect.CodeInvalidArgument, "settings.business_type_invalid")
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := model.AppSetting{
			Key:       common.SettingKeyLowStockThreshold,
			Value:     strconv.FormatInt(int64(req.Msg.LowStockThreshold), 10),
			UpdatedAt: time.Now(),
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).Create(&row).Error; err != nil {
			return err
		}
		if err := common.SetAppTitle(ctx, tx, title); err != nil {
			return err
		}
		// UNSPECIFIED means "don't touch the mode" — so a caller that only edits
		// the threshold can't silently reset a pharmacy shop to retail.
		if bt != settingsifacev1.BussinessType_BUSSINESS_TYPE_UNSPECIFIED {
			return common.SetBussinessType(ctx, tx, int32(bt))
		}
		return nil
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	effective, err := common.GetBussinessType(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.UpdateSettingsResponse{
		Settings: &settingsifacev1.Settings{
			LowStockThreshold: req.Msg.LowStockThreshold,
			AppTitle:          title,
			BusinessType:      settingsifacev1.BussinessType(effective),
		},
	}), nil
}

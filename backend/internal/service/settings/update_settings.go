package settings

import (
	"context"
	"errors"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm/clause"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SettingsService) UpdateSettings(
	ctx context.Context,
	req *connect.Request[settingsifacev1.UpdateSettingsRequest],
) (*connect.Response[settingsifacev1.UpdateSettingsResponse], error) {
	if req.Msg.LowStockThreshold < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("low_stock_threshold must be >= 0"))
	}
	row := model.AppSetting{
		Key:       common.SettingKeyLowStockThreshold,
		Value:     strconv.FormatInt(int64(req.Msg.LowStockThreshold), 10),
		UpdatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).Create(&row).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.UpdateSettingsResponse{
		Settings: &settingsifacev1.Settings{LowStockThreshold: req.Msg.LowStockThreshold},
	}), nil
}

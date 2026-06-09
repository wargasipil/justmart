package common

import (
	"context"
	"errors"
	"strconv"

	"gorm.io/gorm"

	"github.com/justmart/backend/internal/model"
)

const (
	SettingKeyLowStockThreshold = "low_stock_threshold"
	DefaultLowStockThreshold    = int32(10)
)

// GetLowStockThreshold reads the current low-stock threshold from app_settings,
// returning the default (10) when no row exists or the stored value is invalid.
// Shared by SettingsService and ProductService.ListLowStock.
func GetLowStockThreshold(ctx context.Context, db *gorm.DB) (int32, error) {
	var row model.AppSetting
	err := db.WithContext(ctx).Where("key = ?", SettingKeyLowStockThreshold).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DefaultLowStockThreshold, nil
	}
	if err != nil {
		return 0, err
	}
	n, perr := strconv.ParseInt(row.Value, 10, 32)
	if perr != nil {
		return DefaultLowStockThreshold, nil
	}
	return int32(n), nil
}

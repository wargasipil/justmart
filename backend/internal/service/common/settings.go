package common

import (
	"context"
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/justmart/backend/internal/model"
)

const (
	SettingKeyLowStockThreshold = "low_stock_threshold"
	DefaultLowStockThreshold    = int32(10)

	// SettingKeyBussinessType stores the shop's business type as the stringified
	// BussinessType enum value (e.g. "1"). 0 (UNSPECIFIED) when unset.
	SettingKeyBussinessType = "business_type"

	// SettingKeyLicense stores the raw license token entered via the Settings UI
	// (SettingsService.ApplyLicense); SettingKeyLicenseName caches the verified
	// holder name for display. The server re-verifies + applies the stored token
	// on boot when no config/env license is set.
	SettingKeyLicense     = "license"
	SettingKeyLicenseName = "license_name"

	// Default print target (connector mode): which connector device + printer
	// SaleService.PrintReceipt uses when the request carries no explicit target.
	SettingKeyPrintConnectorDevice  = "print_connector_device"
	SettingKeyPrintConnectorPrinter = "print_connector_printer"

	// Business-type enum values, mirroring settings_iface.v1.BussinessType
	// (kept as plain ints so this package stays free of a gen import).
	BussinessTypeUnspecified int32 = 0
	BussinessTypePharmacyShop int32 = 1
	BussinessTypeRetail       int32 = 2
)

// IsPharmacyMode reports whether the shop's configured business type is the
// pharmacy/apotek mode. Pharmacy-only behavior (e.g. POS prescription
// enforcement) keys off this so it's a no-op in retail mode.
func IsPharmacyMode(ctx context.Context, db *gorm.DB) (bool, error) {
	bt, err := GetBussinessType(ctx, db)
	if err != nil {
		return false, err
	}
	return bt == BussinessTypePharmacyShop, nil
}

// GetBussinessType reads the configured business type from app_settings as the
// BussinessType enum's integer value, returning 0 (UNSPECIFIED) when no row
// exists or the stored value is invalid. Returns int32 (not the gen enum) to
// keep this package free of a gen import; callers cast to the proto enum.
func GetBussinessType(ctx context.Context, db *gorm.DB) (int32, error) {
	var row model.AppSetting
	err := db.WithContext(ctx).Where("key = ?", SettingKeyBussinessType).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n, perr := strconv.ParseInt(row.Value, 10, 32)
	if perr != nil {
		return 0, nil
	}
	return int32(n), nil
}

// SetBussinessType upserts the shop's business type into app_settings. Shared by
// SettingsService.SetBussinessSettings and the boot-time license loader (the
// license is the source of truth — it writes the licensed type on every boot).
func SetBussinessType(ctx context.Context, db *gorm.DB, t int32) error {
	return setSetting(ctx, db, SettingKeyBussinessType, strconv.FormatInt(int64(t), 10))
}

// SetLicense persists the raw license token + the verified holder name into
// app_settings (so a UI-applied license survives reboots and is re-applied on
// boot). Shared by SettingsService.ApplyLicense.
func SetLicense(ctx context.Context, db *gorm.DB, token, name string) error {
	if err := setSetting(ctx, db, SettingKeyLicense, token); err != nil {
		return err
	}
	return setSetting(ctx, db, SettingKeyLicenseName, name)
}

// GetLicense returns the stored license token ("" when none was applied via UI).
func GetLicense(ctx context.Context, db *gorm.DB) (string, error) {
	return getSetting(ctx, db, SettingKeyLicense)
}

// GetLicenseName returns the cached licensed-holder name ("" when none).
func GetLicenseName(ctx context.Context, db *gorm.DB) (string, error) {
	return getSetting(ctx, db, SettingKeyLicenseName)
}

// GetPrintTarget returns the saved default print connector device + printer
// ("" each when unset). Read by SaleService.PrintReceipt + SettingsService.
func GetPrintTarget(ctx context.Context, db *gorm.DB) (deviceID, printerName string, err error) {
	deviceID, err = getSetting(ctx, db, SettingKeyPrintConnectorDevice)
	if err != nil {
		return "", "", err
	}
	printerName, err = getSetting(ctx, db, SettingKeyPrintConnectorPrinter)
	if err != nil {
		return "", "", err
	}
	return deviceID, printerName, nil
}

// SetPrintTarget persists the default print connector device + printer.
func SetPrintTarget(ctx context.Context, db *gorm.DB, deviceID, printerName string) error {
	if err := setSetting(ctx, db, SettingKeyPrintConnectorDevice, deviceID); err != nil {
		return err
	}
	return setSetting(ctx, db, SettingKeyPrintConnectorPrinter, printerName)
}

// getSetting reads a single app_settings value ("" when the row is absent).
func getSetting(ctx context.Context, db *gorm.DB, key string) (string, error) {
	var row model.AppSetting
	err := db.WithContext(ctx).Where("key = ?", key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return row.Value, nil
}

// setSetting upserts a single app_settings key/value.
func setSetting(ctx context.Context, db *gorm.DB, key, value string) error {
	row := model.AppSetting{Key: key, Value: value, UpdatedAt: time.Now()}
	return db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).Create(&row).Error
}

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

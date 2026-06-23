package common

import (
	"context"
	"errors"
	"strconv"
	"strings"
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

	// Printed-receipt header (shop name/address) + footer (closing lines), stored
	// as multi-line strings (one receipt line per text line). Seeded at boot from
	// config.yaml printer.header/footer; editable in Settings ▸ Printing.
	SettingKeyReceiptHeader = "receipt_header"
	SettingKeyReceiptFooter = "receipt_footer"

	// Receipt paper width in characters per line (32 = 58mm paper, 48 = 80mm).
	// Seeded at boot from config.yaml printer.width; editable in Settings ▸
	// Printing. Defaults to 32 when unset/invalid.
	SettingKeyReceiptWidth = "receipt_width"
	DefaultReceiptWidth    = int32(32)

	// Payroll statutory config: BPJS contribution rates/caps + PPh21 (TER) tables,
	// stored as JSON blobs. Seeded at boot (set-if-absent) with provisional
	// defaults; editable via PayrollService.SetPayrollSettings.
	SettingKeyPayrollBPJS  = "payroll_bpjs_config"
	SettingKeyPayrollPPh21 = "payroll_pph21_config"

	// Payment-integration config (Xendit-first disbursement gateway). The active
	// provider drives whether payroll can pay via a gateway; the per-provider
	// credentials are applied via Settings ▸ Integrations. config/env overrides
	// win (mirrors the license precedence).
	SettingKeyPaymentActiveProvider = "payment_active_provider"
	SettingKeyXenditAPIKey          = "xendit_api_key"
	SettingKeyXenditWebhookToken    = "xendit_webhook_token"

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

// GetReceiptText returns the stored receipt header + footer (multi-line strings,
// "" when unset). The boot seed populates them from config defaults, so a booted
// server returns the effective values.
func GetReceiptText(ctx context.Context, db *gorm.DB) (header, footer string, err error) {
	header, err = getSetting(ctx, db, SettingKeyReceiptHeader)
	if err != nil {
		return "", "", err
	}
	footer, err = getSetting(ctx, db, SettingKeyReceiptFooter)
	if err != nil {
		return "", "", err
	}
	return header, footer, nil
}

// SetReceiptText persists the receipt header + footer (multi-line strings).
func SetReceiptText(ctx context.Context, db *gorm.DB, header, footer string) error {
	if err := setSetting(ctx, db, SettingKeyReceiptHeader, header); err != nil {
		return err
	}
	return setSetting(ctx, db, SettingKeyReceiptFooter, footer)
}

// GetReceiptWidth returns the configured receipt paper width (chars per line),
// falling back to DefaultReceiptWidth (32) when unset or invalid. Read by
// SaleService.PrintReceipt and SettingsService.GetReceiptSettings.
func GetReceiptWidth(ctx context.Context, db *gorm.DB) (int32, error) {
	v, err := getSetting(ctx, db, SettingKeyReceiptWidth)
	if err != nil {
		return 0, err
	}
	n, perr := strconv.ParseInt(strings.TrimSpace(v), 10, 32)
	if perr != nil || n <= 0 {
		return DefaultReceiptWidth, nil
	}
	return int32(n), nil
}

// SetReceiptWidth persists the receipt paper width (chars per line). Values <= 0
// are coerced to DefaultReceiptWidth so a bad input can't produce an empty line.
func SetReceiptWidth(ctx context.Context, db *gorm.DB, width int32) error {
	if width <= 0 {
		width = DefaultReceiptWidth
	}
	return setSetting(ctx, db, SettingKeyReceiptWidth, strconv.FormatInt(int64(width), 10))
}

// SeedReceiptWidth writes the config-derived paper width into app_settings ONLY
// when no row exists yet (mirrors SeedReceiptDefaults), so a config.yaml-based
// shop keeps its width and a later edit is never overwritten on reboot. A
// non-positive config width seeds the default.
func SeedReceiptWidth(ctx context.Context, db *gorm.DB, width int) error {
	w := int32(width)
	if w <= 0 {
		w = DefaultReceiptWidth
	}
	return seedIfAbsent(ctx, db, SettingKeyReceiptWidth, strconv.FormatInt(int64(w), 10))
}

// GetPayrollConfig returns the stored BPJS + PPh21 config JSON ("" each when
// unset). The payroll package falls back to its baked defaults on empty/parse
// error, so an unseeded shop still computes statutory amounts.
func GetPayrollConfig(ctx context.Context, db *gorm.DB) (bpjs, pph21 string, err error) {
	bpjs, err = getSetting(ctx, db, SettingKeyPayrollBPJS)
	if err != nil {
		return "", "", err
	}
	pph21, err = getSetting(ctx, db, SettingKeyPayrollPPh21)
	if err != nil {
		return "", "", err
	}
	return bpjs, pph21, nil
}

// SetPayrollConfig persists the BPJS + PPh21 config JSON (PayrollService.SetPayrollSettings).
func SetPayrollConfig(ctx context.Context, db *gorm.DB, bpjs, pph21 string) error {
	if err := setSetting(ctx, db, SettingKeyPayrollBPJS, bpjs); err != nil {
		return err
	}
	return setSetting(ctx, db, SettingKeyPayrollPPh21, pph21)
}

// SeedPayrollDefaults writes the provisional payroll config JSON into
// app_settings ONLY when absent (mirrors SeedReceiptDefaults), so a later edit is
// never overwritten on reboot. The default JSON is supplied by the payroll
// package (which owns the config structs).
func SeedPayrollDefaults(ctx context.Context, db *gorm.DB, defBPJS, defPPh21 string) error {
	if err := seedIfAbsent(ctx, db, SettingKeyPayrollBPJS, defBPJS); err != nil {
		return err
	}
	return seedIfAbsent(ctx, db, SettingKeyPayrollPPh21, defPPh21)
}

// GetActiveProvider returns the active payment provider ("" = manual-only).
func GetActiveProvider(ctx context.Context, db *gorm.DB) (string, error) {
	return getSetting(ctx, db, SettingKeyPaymentActiveProvider)
}

// SetActiveProvider persists which payment gateway payroll uses ("" = manual).
func SetActiveProvider(ctx context.Context, db *gorm.DB, provider string) error {
	return setSetting(ctx, db, SettingKeyPaymentActiveProvider, provider)
}

// GetXenditCredentials returns the stored Xendit api key + webhook token ("" each
// when unset). Callers prefer config/env over these (license-style precedence).
func GetXenditCredentials(ctx context.Context, db *gorm.DB) (apiKey, webhookToken string, err error) {
	apiKey, err = getSetting(ctx, db, SettingKeyXenditAPIKey)
	if err != nil {
		return "", "", err
	}
	webhookToken, err = getSetting(ctx, db, SettingKeyXenditWebhookToken)
	if err != nil {
		return "", "", err
	}
	return apiKey, webhookToken, nil
}

// SetXenditCredentials persists the Xendit api key + webhook token.
func SetXenditCredentials(ctx context.Context, db *gorm.DB, apiKey, webhookToken string) error {
	if err := setSetting(ctx, db, SettingKeyXenditAPIKey, apiKey); err != nil {
		return err
	}
	return setSetting(ctx, db, SettingKeyXenditWebhookToken, webhookToken)
}

// SeedPaymentDefaults seeds the active provider from config ONLY when absent
// (set-if-absent), so a UI change is never overwritten on reboot.
func SeedPaymentDefaults(ctx context.Context, db *gorm.DB, defActiveProvider string) error {
	if defActiveProvider == "" {
		return nil
	}
	return seedIfAbsent(ctx, db, SettingKeyPaymentActiveProvider, defActiveProvider)
}

// ReceiptLines splits a stored multi-line header/footer string into receipt
// lines: normalizes CRLF, trims leading/trailing blank lines, keeps interior
// ones. Empty input → no lines.
func ReceiptLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.Trim(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// SeedReceiptDefaults writes the config-derived header/footer into app_settings
// ONLY when no row exists yet — so an existing config.yaml-based shop keeps its
// header on first boot, and a user's later edit (incl. clearing it) is never
// overwritten on subsequent boots.
func SeedReceiptDefaults(ctx context.Context, db *gorm.DB, defHeader, defFooter []string) error {
	if err := seedIfAbsent(ctx, db, SettingKeyReceiptHeader, strings.Join(defHeader, "\n")); err != nil {
		return err
	}
	return seedIfAbsent(ctx, db, SettingKeyReceiptFooter, strings.Join(defFooter, "\n"))
}

func seedIfAbsent(ctx context.Context, db *gorm.DB, key, value string) error {
	var row model.AppSetting
	err := db.WithContext(ctx).Where("key = ?", key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return setSetting(ctx, db, key, value)
	}
	return err
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

// Package settings implements settings_iface.v1.SettingsService (app-wide
// key/value settings). The shared low-stock-threshold reader lives in
// service/common (also used by ProductService.ListLowStock).
package settings

import "gorm.io/gorm"

type SettingsService struct {
	db   *gorm.DB
	mode string // config connector.mode ("" => tcp); set via SetConnectorMode
}

func NewSettingsService(db *gorm.DB) *SettingsService { return &SettingsService{db: db} }

// SetConnectorMode records the configured print mode (config connector.mode) so
// GetPrintingInfo can report it to the Settings ▸ Printing panel. Called once
// from serve.go; defaults to "" (treated as tcp) when unset.
func (s *SettingsService) SetConnectorMode(mode string) { s.mode = mode }

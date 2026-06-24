// Package settings implements settings_iface.v1.SettingsService (app-wide
// key/value settings). The shared low-stock-threshold reader lives in
// service/common (also used by ProductService.ListLowStock).
package settings

import (
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/config"
)

type SettingsService struct {
	db   *gorm.DB
	mode string // config connector.mode ("" => tcp); set via SetConnectorMode

	// Autoupdater wiring (set via SetUpdate from serve.go). updateAPIBase is the
	// GitHub API root, overridable in tests to point at an httptest server.
	version       string
	updateCfg     config.Update
	updateAPIBase string
}

func NewSettingsService(db *gorm.DB) *SettingsService {
	return &SettingsService{db: db, updateAPIBase: "https://api.github.com"}
}

// SetConnectorMode records the configured print mode (config connector.mode) so
// GetPrintingInfo can report it to the Settings ▸ Printing panel. Called once
// from serve.go; defaults to "" (treated as tcp) when unset.
func (s *SettingsService) SetConnectorMode(mode string) { s.mode = mode }

// SetUpdate records the running build version + update config for the
// autoupdater RPCs (CheckUpdate / ApplyUpdate). Called once from serve.go.
func (s *SettingsService) SetUpdate(version string, cfg config.Update) {
	s.version = version
	s.updateCfg = cfg
}

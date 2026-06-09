// Package settings implements settings_iface.v1.SettingsService (app-wide
// key/value settings). The shared low-stock-threshold reader lives in
// service/common (also used by ProductService.ListLowStock).
package settings

import "gorm.io/gorm"

type SettingsService struct {
	db *gorm.DB
}

func NewSettingsService(db *gorm.DB) *SettingsService { return &SettingsService{db: db} }

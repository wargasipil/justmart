// Package health implements health_iface.v1.HealthService.
package health

import "gorm.io/gorm"

type HealthService struct {
	db *gorm.DB
}

func NewHealthService(db *gorm.DB) *HealthService { return &HealthService{db: db} }

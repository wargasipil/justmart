// Package analytics implements analytics_iface.v1.AnalyticsService — three
// dimension-scoped RPCs (DailyMetric, ProductMetric, UserMetric), one per file.
// Pure helpers live in helpers.go. Pattern: phase 1 = sort + paginate to get
// IDs; phase 2 = fetch metric blocks for those IDs (N+1-free; the frontend
// resolves names via Resolve<Domain>(ids)).
package analytics

import "gorm.io/gorm"

type AnalyticsService struct {
	db *gorm.DB
}

func NewAnalyticsService(db *gorm.DB) *AnalyticsService { return &AnalyticsService{db: db} }

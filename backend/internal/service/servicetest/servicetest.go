// Package servicetest provides shared fixtures for the co-located ConnectRPC
// handler unit tests (one <rpc>_test.go per handler). Each test gets a fresh,
// migrated, throwaway SQLite database bound to its *testing.T, plus helpers for
// building a test config and injecting an authenticated principal.
//
// It reuses db.Open (UUID create-callback + WAL/FK pragmas + single-writer pool)
// and dbmigrate.Run (embedded goose set) so tests exercise the same DB setup as
// production. It imports only auth/config/db/dbmigrate/model/service-user, none
// of which import a sibling service package, so there is no import cycle from any
// service/<domain> test.
package servicetest

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/db"
	"github.com/justmart/backend/internal/dbmigrate"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/user"
)

// Fixed bootstrap-owner credentials used by tests that need a logged-in owner.
// Distinct from config.yaml so unit tests never depend on a checked-in secret.
const (
	OwnerEmail    = "owner@test.local"
	OwnerPassword = "owner-password-123"
	jwtSecret     = "test-jwt-secret-0123456789abcdef-not-for-prod" // >=32 bytes for HS256
)

// NewConfig returns a minimal *config.Config wired for an on-disk SQLite file
// inside t.TempDir(). Suitable for services that need cfg (sale takes
// cfg.Printer, backup takes the whole cfg). The DB file path is unique per test,
// so tests are isolated and may run in parallel.
func NewConfig(t *testing.T) *config.Config {
	t.Helper()
	autoMigrate := false // tests migrate explicitly via NewDB.
	return &config.Config{
		Database: config.Database{
			Driver:      "sqlite",
			Path:        filepath.Join(t.TempDir(), "test.sqlite"),
			AutoMigrate: &autoMigrate,
		},
		Auth: config.Auth{
			JWTSecret:       jwtSecret,
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: 24 * time.Hour,
		},
		Bootstrap: config.Bootstrap{
			OwnerEmail:    OwnerEmail,
			OwnerPassword: OwnerPassword,
		},
		Printer: config.Printer{Enabled: false},
	}
}

// NewDB opens a fresh SQLite database for cfg and applies all migrations. The
// connection pool is closed via t.Cleanup. Use NewDB when a service only needs
// *gorm.DB; use New when you also need the cfg.
func NewDB(t *testing.T, cfg *config.Config) *gorm.DB {
	t.Helper()
	gormDB, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("sql.DB handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := dbmigrate.Run(sqlDB, cfg.Database.DriverName()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return gormDB
}

// New is the common case: build a test config and open+migrate its DB in one
// call. Returns both because cfg-taking services (sale, backup, auth issuers)
// need the config alongside the gorm handle.
func New(t *testing.T) (*gorm.DB, *config.Config) {
	t.Helper()
	cfg := NewConfig(t)
	return NewDB(t, cfg), cfg
}

// EnsureOwner upserts the servicetest bootstrap owner into gormDB and returns the
// owner's user id. It also grants the owner the default-warehouse membership
// (EnsureBootstrapOwner does this), so warehouse-scoped RPCs resolve to MAIN.
// Call this from tests that need a real, logged-in user.
func EnsureOwner(t *testing.T, gormDB *gorm.DB, cfg *config.Config) string {
	t.Helper()
	if err := user.NewUserService(gormDB).
		EnsureBootstrapOwner(context.Background(), cfg.Bootstrap); err != nil {
		t.Fatalf("ensure bootstrap owner: %v", err)
	}
	var u model.User
	if err := gormDB.Where("email = ?", cfg.Bootstrap.OwnerEmail).First(&u).Error; err != nil {
		t.Fatalf("load bootstrap owner: %v", err)
	}
	return u.ID
}

// CtxAs returns a context carrying an auth.Principal with the given role and user
// id (no warehouse — handlers fall back to the migration-seeded MAIN warehouse
// via common.ResolveWarehouse). Role is "OWNER","PHARMACIST", or "CASHIER".
func CtxAs(ctx context.Context, role, userID string) context.Context {
	return auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Role: role})
}

// CtxInWarehouse is the warehouse-scoped variant for the rare RPC that must run
// against a specific (non-default) warehouse. Most tests don't need this — the
// seeded MAIN warehouse is the ResolveWarehouse fallback.
func CtxInWarehouse(ctx context.Context, role, userID, warehouseID string) context.Context {
	return auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Role: role, WarehouseID: warehouseID})
}

// OwnerCtx is shorthand for an OWNER principal. Pair with EnsureOwner to inject
// the real owner id (some handlers FK the caller, e.g. sale.cashier_user_id ->
// users.id, so a random uuid would violate the constraint).
func OwnerCtx(ctx context.Context, userID string) context.Context {
	return CtxAs(ctx, "OWNER", userID)
}

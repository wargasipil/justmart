// Package servicetest provides shared fixtures for the co-located ConnectRPC
// handler unit tests (one <rpc>_test.go per handler). Each test gets a fresh,
// migrated, throwaway database bound to its *testing.T, plus helpers for
// building a test config and injecting an authenticated principal.
//
// By default the engine is SQLite (one temp file per test). Set
// JUSTMART_TEST_DB_DRIVER=postgres to run the SAME tests against an isolated
// per-test Postgres SCHEMA on the dev Postgres (make up). The test files never
// change — New/NewDB/NewConfig branch on the env var internally. Run both via
// `make test-unit-all`.
//
// It reuses db.Open (SQLite path) and dbmigrate.Run (embedded goose set) so
// tests exercise the same DB setup as production. It imports only
// auth/config/db/dbmigrate/model/service-user, none of which import a sibling
// service package, so there is no import cycle from any service/<domain> test.
package servicetest

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"gorm.io/driver/postgres"
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

	// envDriver selects the test engine. Empty/"sqlite" => SQLite (default,
	// fast, no dependency). "postgres" => isolated per-test schema on dev PG.
	envDriver = "JUSTMART_TEST_DB_DRIVER"
)

// schemaSeq makes per-test schema names unique within a process; combined with
// the PID it's unique across the concurrently-running package test binaries
// (`go test -p N`).
var schemaSeq atomic.Int64

// usePostgres reports whether tests should run against Postgres.
func usePostgres() bool {
	return strings.EqualFold(os.Getenv(envDriver), "postgres")
}

// pgEnv reads the dev-Postgres connection settings from the environment, with
// docker-compose.yml defaults. servicetest owns these reads (rather than
// config.applyEnvOverrides) so the production config struct stays untouched and
// PORT/USER/NAME are overridable here without widening prod env support.
func pgEnv() config.Database {
	return config.Database{
		Driver:   "postgres",
		Host:     envOr("JUSTMART_DB_HOST", "localhost"),
		Port:     atoiOr("JUSTMART_DB_PORT", 5432),
		User:     envOr("JUSTMART_DB_USER", "justmart"),
		Password: envOr("JUSTMART_DB_PASSWORD", "justmart"),
		Name:     envOr("JUSTMART_DB_NAME", "justmart"),
		SSLMode:  envOr("JUSTMART_DB_SSLMODE", "disable"),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func atoiOr(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

// NewConfig returns a minimal *config.Config wired for the active test engine.
// SQLite (default): an on-disk file inside t.TempDir(). Postgres: the dev PG
// creds (the per-test schema is applied later, in NewDB). Suitable for services
// that need cfg (sale takes cfg.Printer, backup takes the whole cfg). DBs are
// isolated per test, so tests are isolated and may run in parallel.
func NewConfig(t *testing.T) *config.Config {
	t.Helper()
	autoMigrate := false // tests migrate explicitly via NewDB.

	dbCfg := config.Database{
		Driver:      "sqlite",
		Path:        filepath.Join(t.TempDir(), "test.sqlite"),
		AutoMigrate: &autoMigrate,
	}
	if usePostgres() {
		dbCfg = pgEnv()
		dbCfg.AutoMigrate = &autoMigrate
	}

	return &config.Config{
		Database: dbCfg,
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

// NewDB opens a fresh database for cfg and applies all migrations. The
// connection pool (and, on Postgres, the throwaway schema) is torn down via
// t.Cleanup. Use NewDB when a service only needs *gorm.DB; use New when you also
// need the cfg.
func NewDB(t *testing.T, cfg *config.Config) *gorm.DB {
	t.Helper()
	if usePostgres() {
		return newPostgresDB(t, cfg)
	}
	return newSQLiteDB(t, cfg)
}

// newSQLiteDB is the original NewDB body, preserved byte-for-byte.
func newSQLiteDB(t *testing.T, cfg *config.Config) *gorm.DB {
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

// citextOnce ensures the citext extension is installed in public exactly once
// per process (parallel package binaries each run it once; the race is handled).
var citextOnce sync.Once

// ensureCitextPublic installs citext into the public schema once. The Postgres
// migrations do `CREATE EXTENSION IF NOT EXISTS citext` (00001); with a per-test
// schema search_path that would create the extension in the test schema and
// DROP it on cleanup, which races and breaks concurrent tests. Pre-creating it
// in public makes every migration's IF NOT EXISTS a global no-op, and each test
// schema resolves the citext type via the ",public" in its search_path.
func ensureCitextPublic(t *testing.T, baseDSN string) {
	t.Helper()
	citextOnce.Do(func() {
		boot, err := sql.Open("pgx", baseDSN)
		if err != nil {
			t.Fatalf("citext bootstrap open: %v", err)
		}
		defer boot.Close()
		if _, err := boot.ExecContext(context.Background(),
			`CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public`); err != nil {
			// A parallel package binary may have created it concurrently — verify
			// it now exists rather than failing on the create race.
			var n int
			if qerr := boot.QueryRowContext(context.Background(),
				`SELECT count(*) FROM pg_extension WHERE extname = 'citext'`).Scan(&n); qerr != nil || n == 0 {
				t.Fatalf("ensure citext in public: %v", err)
			}
		}
	})
}

// schemaNameRE strips everything that isn't a valid lowercase PG identifier char.
var schemaNameRE = regexp.MustCompile(`[^a-z0-9_]+`)

// schemaName builds a unique, valid Postgres identifier from t.Name() plus the
// PID and a process-local atomic counter. t.Name() (e.g. "TestCreateCustomer/ok")
// is sanitized to lowercase [a-z0-9_] and truncated so the final name stays well
// under PG's 63-byte identifier limit. The PID disambiguates concurrent package
// test binaries (`go test -p N`); the atomic disambiguates within a process.
func schemaName(t *testing.T) string {
	base := strings.ToLower(t.Name())
	base = schemaNameRE.ReplaceAllString(base, "_")
	base = strings.Trim(base, "_")
	if len(base) > 40 {
		base = base[:40]
	}
	return fmt.Sprintf("t_%s_%d_%d", base, os.Getpid(), schemaSeq.Add(1))
}

// newPostgresDB gives the test its own throwaway Postgres SCHEMA on the dev
// cluster:
//
//  1. A short-lived bootstrap *sql.DB CREATEs the schema.
//  2. The test gorm pool is opened with options=-c search_path=<schema>,public
//     so every statement (incl. goose's) targets the schema; ,public keeps the
//     citext type (created in public by migration 00001) resolvable.
//  3. dbmigrate.Run creates all tables + goose_db_version INSIDE the schema.
//  4. t.Cleanup closes the pool, then DROP SCHEMA ... CASCADE via a bootstrap
//     conn (registered FIRST so it runs LAST — after the pool close, since the
//     drop blocks on open sessions).
func newPostgresDB(t *testing.T, cfg *config.Config) *gorm.DB {
	t.Helper()
	name := schemaName(t)
	baseDSN := cfg.Database.DSN() // host=.. port=.. ... sslmode=..  (no search_path)

	// 0. Ensure citext lives in public ONCE (see ensureCitextPublic) so every
	//    per-test schema resolves it via ",public" instead of each migration
	//    trying to CREATE — and each DROP SCHEMA dropping — its own copy, which
	//    races and breaks under parallel tests.
	ensureCitextPublic(t, baseDSN)

	// 1. Bootstrap connection on the base DB to create the schema.
	boot, err := sql.Open("pgx", baseDSN)
	if err != nil {
		t.Fatalf("open bootstrap pg: %v", err)
	}
	if _, err := boot.ExecContext(context.Background(),
		fmt.Sprintf(`CREATE SCHEMA %s`, quoteIdent(name))); err != nil {
		_ = boot.Close()
		t.Fatalf("create schema %q: %v", name, err)
	}
	_ = boot.Close()

	// LIFO cleanup: register the schema drop FIRST so it runs LAST (after the
	// pool is closed below — DROP SCHEMA blocks on open sessions).
	t.Cleanup(func() { dropSchema(t, baseDSN, name) })

	// 2. Test pool pinned to the schema via the libpq `options` connection param
	//    (-c search_path=...). The value contains a space (-c<space>search_path),
	//    so it MUST be single-quoted in the space-separated keyword DSN; otherwise
	//    the DSN parser splits it and the backend gets a bare "-c" and rejects the
	//    connection. ,public keeps the citext type (in public) resolvable.
	dsn := baseDSN + " options='-c search_path=" + name + ",public'"
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open pg schema %q: %v", name, err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("sql.DB handle: %v", err)
	}
	// Cap the pool: each test owns a schema full of ~40 tables, and every
	// CREATE/DROP SCHEMA holds many locks — too many concurrent test pools
	// exhaust PG's shared lock table. Keep it tiny (tests are single-threaded
	// per test); the Makefile also bounds parallelism (-p 1 -parallel 4).
	sqlDB.SetMaxOpenConns(2)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Minute)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlDB.PingContext(context.Background()); err != nil {
		t.Fatalf("ping pg schema %q: %v", name, err)
	}

	// 3. Migrate INTO the schema (goose writes goose_db_version here too).
	if err := dbmigrate.Run(sqlDB, cfg.Database.DriverName()); err != nil {
		t.Fatalf("migrate pg schema %q: %v", name, err)
	}
	return gormDB
}

// dropSchema removes the throwaway test schema via a fresh bootstrap connection.
// Best-effort: logs (does not fail) so a cleanup hiccup can't mask the test's
// own result.
func dropSchema(t *testing.T, baseDSN, name string) {
	t.Helper()
	boot, err := sql.Open("pgx", baseDSN)
	if err != nil {
		t.Logf("drop schema %q: open bootstrap: %v", name, err)
		return
	}
	defer boot.Close()
	if _, err := boot.ExecContext(context.Background(),
		fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, quoteIdent(name))); err != nil {
		t.Logf("drop schema %q: %v", name, err)
	}
}

// quoteIdent double-quotes a PG identifier (our names are already [a-z0-9_], so
// this is belt-and-suspenders).
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
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

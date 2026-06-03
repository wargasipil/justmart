package db

import (
	"context"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/config"
)

func Open(cfg *config.Config) (*gorm.DB, error) {
	if cfg.Database.IsSQLite() {
		return openSQLite(cfg)
	}
	return openPostgres(cfg)
}

func openPostgres(cfg *config.Config) (*gorm.DB, error) {
	gormDB, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}
	if err := sqlDB.PingContext(context.Background()); err != nil {
		return nil, err
	}
	return gormDB, nil
}

func openSQLite(cfg *config.Config) (*gorm.DB, error) {
	gormDB, err := gorm.Open(sqlite.Open(cfg.Database.SQLiteDSN()), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}
	// SQLite is a single-writer engine. Pinning the pool to one connection makes
	// every write transaction strictly serial, which is exactly what the
	// stock-consuming paths need now that FOR UPDATE row locks are no-ops on
	// SQLite (see service.rowLock). For a single-PC turnkey deploy throughput is
	// a non-issue; correctness (no oversell, no "database is locked") is.
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.PingContext(context.Background()); err != nil {
		return nil, err
	}
	// Postgres fills UUID primary keys via the gen_random_uuid() column default;
	// SQLite has no such function, so generate ids application-side before insert.
	if err := registerUUIDDefault(gormDB); err != nil {
		return nil, err
	}
	return gormDB, nil
}

func MustOpen(cfg *config.Config) *gorm.DB {
	db, err := Open(cfg)
	if err != nil {
		panic(err)
	}
	return db
}

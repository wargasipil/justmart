package warga_event

import (
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open opens a SQLite database with the warga_event schema migrated. dsn is a
// glebarez/sqlite DSN — a file path, or ":memory:" for an ephemeral store.
//
// The connection pool is pinned to a single connection. SQLite is a
// single-writer engine and (for ":memory:") a pool of >1 connection would
// expose independent in-memory databases; one connection keeps every query
// against the same store and serializes writes, which is the correctness model
// this broker relies on instead of row locks.
func Open(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(applyPragmas(dsn)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)

	if err := Migrate(db); err != nil {
		return nil, err
	}
	return db, nil
}

// Migrate creates/updates the broker tables. Safe to call repeatedly.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Topic{}, &Subscription{}, &Delivery{})
}

// applyPragmas appends connection pragmas to a file-backed DSN: busy_timeout
// makes a transient external write lock block-and-retry rather than failing
// immediately, and WAL lets readers run during a write. In-memory DSNs (and any
// DSN that already carries a query string) are left untouched — the
// single-connection pool already serializes in-memory access. Uses the
// modernc/glebarez "?_pragma=NAME(VALUE)" form.
func applyPragmas(dsn string) string {
	if dsn == "" || strings.Contains(dsn, ":memory:") || strings.Contains(dsn, "?") {
		return dsn
	}
	return dsn + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
}

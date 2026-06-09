package common

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// This file centralizes the handful of SQL constructs that differ between the
// two supported engines (Postgres, SQLite). Everything keys off the GORM
// dialector name so service code stays uniform.

func IsSQLite(db *gorm.DB) bool { return db.Dialector.Name() == "sqlite" }

// RowLock applies a SELECT ... FOR UPDATE row lock on Postgres so a
// read-check-insert can serialize per row. SQLite has no row-level locking;
// correctness there comes from the single-writer connection pool configured in
// db.openSQLite, so the lock is a no-op and we return the query unchanged.
func RowLock(tx *gorm.DB) *gorm.DB {
	if IsSQLite(tx) {
		return tx
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"})
}

// LikeOp returns the case-insensitive LIKE operator for the dialect. Postgres
// has ILIKE; SQLite's LIKE is already case-insensitive for ASCII (which is what
// every Search* filter relies on).
func LikeOp(db *gorm.DB) string {
	if IsSQLite(db) {
		return "LIKE"
	}
	return "ILIKE"
}

// EpochExpr yields a SQL expression returning the Unix epoch (seconds, integer)
// for the given timestamp column/expression.
func EpochExpr(db *gorm.DB, col string) string {
	if IsSQLite(db) {
		return fmt.Sprintf("CAST(strftime('%%s', %s) AS INTEGER)", col)
	}
	return fmt.Sprintf("EXTRACT(EPOCH FROM %s)::bigint", col)
}

// DayKeyExpr yields a SQL expression that formats a timestamp column into a
// 'YYYY-MM-DD' day-key string. Callers GROUP BY this and fold the per-day rows
// into the requested granularity (week/month) in Go — portable across engines
// and avoids Postgres-only DATE_TRUNC / SQLite ISO-week gymnastics.
func DayKeyExpr(db *gorm.DB, col string) string {
	if IsSQLite(db) {
		return fmt.Sprintf("strftime('%%Y-%%m-%%d', %s)", col)
	}
	return fmt.Sprintf("to_char(%s, 'YYYY-MM-DD')", col)
}

// DateAddNowDays yields a SQL date expression for "today + n days" (used for the
// 30-day expiry window). CURRENT_DATE works as the lower bound on both engines.
func DateAddNowDays(db *gorm.DB, n int) string {
	if IsSQLite(db) {
		return fmt.Sprintf("date('now', '+%d days')", n)
	}
	return fmt.Sprintf("(CURRENT_DATE + INTERVAL '%d days')", n)
}

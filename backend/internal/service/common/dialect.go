package common

import (
	"fmt"
	"time"

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

// LocalDayKeyExpr is DayKeyExpr for TIMESTAMP columns: it renders the day key in
// the server's local zone (time.Local), which is what the Go side assumes — day
// strings come back through time.ParseInLocation(..., time.Local) and the bucket
// list is enumerated from local midnights.
//
// Plain DayKeyExpr formats an instant in UTC on both engines (SQLite's strftime
// normalizes the stored offset away; Postgres renders a TIMESTAMPTZ in the
// session TimeZone, UTC in the shipped containers). For a shop at UTC+7 that put
// every sale between 00:00 and 07:00 local into the PREVIOUS day's bucket. Use
// this for timestamps; keep DayKeyExpr for DATE columns (expiry_date,
// valid_from/valid_until) — those carry no zone and must not be shifted.
//
// The offset is resolved from time.Local at call time. Indonesia has no DST; in
// a DST zone a row within an hour of a transition can land on the neighbouring
// day — still far better than being a whole day off.
func LocalDayKeyExpr(db *gorm.DB, col string) string {
	_, offset := time.Now().In(time.Local).Zone()
	if IsSQLite(db) {
		return fmt.Sprintf("strftime('%%Y-%%m-%%d', %s, '%+d seconds')", col, offset)
	}
	return fmt.Sprintf("to_char((%s AT TIME ZONE 'UTC') + INTERVAL '%d seconds', 'YYYY-MM-DD')", col, offset)
}

// DateAddNowDays yields a SQL date expression for "today + n days" (used for the
// 30-day expiry window). CURRENT_DATE works as the lower bound on both engines.
func DateAddNowDays(db *gorm.DB, n int) string {
	if IsSQLite(db) {
		return fmt.Sprintf("date('now', '+%d days')", n)
	}
	return fmt.Sprintf("(CURRENT_DATE + INTERVAL '%d days')", n)
}

package dbmigrate_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/go-sqlite" // pure-Go "sqlite" database/sql driver
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/migrations"
)

// TestSQLite_UpgradeFromV1 proves the SQLite incremental migrations upgrade an
// EXISTING database (one that only ever applied the consolidated baseline,
// version 1) in place — without a reset, preserving data. This is the exact
// scenario that produced "table sales has no column named prescription_id":
// editing 00001 doesn't touch an already-applied DB, so 00035/00036 must carry
// the deltas. Also verifies the 00035 users-table rebuild keeps existing rows
// and widens the role CHECK to accept APOTEKER.
func TestSQLite_UpgradeFromV1(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "upgrade.sqlite")
	dsn := (config.Database{Driver: "sqlite", Path: dbPath}).SQLiteDSN()
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1) // single-writer; the NO-TRANSACTION rebuild needs one conn

	require.NoError(t, goose.SetDialect("sqlite3"))
	goose.SetBaseFS(migrations.FS("sqlite"))

	// Simulate an existing DB: apply ONLY the consolidated baseline (version 1).
	require.NoError(t, goose.UpTo(db, ".", 1))
	require.False(t, columnExists(t, db, "sales", "prescription_id"), "baseline should lack prescription_id")

	// Seed a user — must survive the 00035 users-table rebuild.
	_, err = db.Exec(`INSERT INTO users (id, email, password_hash, role) VALUES ('u1','owner@x.test','h','OWNER')`)
	require.NoError(t, err)

	// Apply the incrementals (00035 rebuild + 00036 prescriptions).
	require.NoError(t, goose.Up(db, "."))

	// Schema upgraded in place.
	require.True(t, columnExists(t, db, "sales", "prescription_id"), "prescription_id added by 00036")
	require.True(t, tableExists(t, db, "prescriptions"))
	require.True(t, tableExists(t, db, "prescription_items"))
	require.True(t, tableExists(t, db, "rx_no_counters"))

	// Data preserved across the users rebuild.
	var email string
	require.NoError(t, db.QueryRow(`SELECT email FROM users WHERE id='u1'`).Scan(&email))
	require.Equal(t, "owner@x.test", email)

	// CHECK widened: APOTEKER now insertable; a bogus role still rejected.
	_, err = db.Exec(`INSERT INTO users (id, email, password_hash, role) VALUES ('u2','apo@x.test','h','APOTEKER')`)
	require.NoError(t, err, "APOTEKER must be allowed after 00035")
	_, err = db.Exec(`INSERT INTO users (id, email, password_hash, role) VALUES ('u3','bad@x.test','h','BOGUS')`)
	require.Error(t, err, "unknown role must still violate the CHECK")
}

// TestSQLite_UpgradeTo00043_PurchaseReturn proves an EXISTING SQLite DB (already
// migrated to 00042) upgrades in place to 00043 — the riskiest part being the
// stock_movements table-rebuild that widens the type CHECK to accept
// 'PURCHASE_RETURN'. It asserts the new column/tables land and that the rebuilt
// CHECK accepts PURCHASE_RETURN while still rejecting an unknown type.
func TestSQLite_UpgradeTo00043_PurchaseReturn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "upgrade43.sqlite")
	dsn := (config.Database{Driver: "sqlite", Path: dbPath}).SQLiteDSN()
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1) // single-writer; the NO-TRANSACTION rebuild needs one conn

	require.NoError(t, goose.SetDialect("sqlite3"))
	goose.SetBaseFS(migrations.FS("sqlite"))

	// Simulate a DB already migrated to just before this change (version 42).
	require.NoError(t, goose.UpTo(db, ".", 42))
	require.False(t, columnExists(t, db, "purchase_orders", "returned_amount"),
		"returned_amount should not exist before 00043")
	require.False(t, tableExists(t, db, "purchase_returns"), "purchase_returns should not exist before 00043")

	// Apply 00043 in place.
	require.NoError(t, goose.Up(db, "."))

	require.True(t, columnExists(t, db, "purchase_orders", "returned_amount"), "returned_amount added by 00043")
	require.True(t, tableExists(t, db, "purchase_returns"))
	require.True(t, tableExists(t, db, "purchase_return_items"))
	require.True(t, tableExists(t, db, "rtn_no_counters"))

	// The rebuilt stock_movements CHECK accepts PURCHASE_RETURN (FKs off so we
	// isolate the CHECK, not reference integrity).
	_, err = db.Exec(`PRAGMA foreign_keys=OFF`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO stock_movements (id, batch_id, qty, type, user_id, warehouse_id)
		VALUES ('m-ret','b1',-5,'PURCHASE_RETURN','u1','w1')`)
	require.NoError(t, err, "PURCHASE_RETURN must be allowed after 00043")
	_, err = db.Exec(`INSERT INTO stock_movements (id, batch_id, qty, type, user_id, warehouse_id)
		VALUES ('m-bad','b1',-5,'BOGUS','u1','w1')`)
	require.Error(t, err, "unknown movement type must still violate the CHECK")
}

func columnExists(t *testing.T, db *sql.DB, table, col string) bool {
	t.Helper()
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		require.NoError(t, rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk))
		if name == col {
			return true
		}
	}
	return false
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n))
	return n == 1
}

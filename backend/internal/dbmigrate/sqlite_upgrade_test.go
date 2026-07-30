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

// TestSQLite_UpgradeTo00045_DiscountPerItem proves an existing SQLite DB
// (migrated to 00044) gains the additive purchase_order_items.discount_per_item
// column in place and accepts a row that sets it — no table rebuild involved.
func TestSQLite_UpgradeTo00045_DiscountPerItem(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "upgrade45.sqlite")
	dsn := (config.Database{Driver: "sqlite", Path: dbPath}).SQLiteDSN()
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	require.NoError(t, goose.SetDialect("sqlite3"))
	goose.SetBaseFS(migrations.FS("sqlite"))

	require.NoError(t, goose.UpTo(db, ".", 44))
	require.False(t, columnExists(t, db, "purchase_order_items", "discount_per_item"),
		"discount_per_item should not exist before 00045")

	require.NoError(t, goose.Up(db, "."))
	require.True(t, columnExists(t, db, "purchase_order_items", "discount_per_item"),
		"discount_per_item added by 00045")

	// A row setting the flag inserts cleanly (FKs off to isolate the column).
	_, err = db.Exec(`PRAGMA foreign_keys=OFF`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO purchase_order_items
		(id, purchase_order_id, product_id, ordered_qty, unit_cost_price, subtotal, discount_per_item)
		VALUES ('poi-1','po-1','p-1',1,100,100,1)`)
	require.NoError(t, err, "row with discount_per_item must insert after 00045")
}

// TestSQLite_UpgradeTo00047_RestockDiscountPerItem proves an existing SQLite DB
// (migrated to 00046) gains the additive per-item discount columns on both
// restock-history tables in place — no table rebuild.
func TestSQLite_UpgradeTo00047_RestockDiscountPerItem(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "upgrade47.sqlite")
	dsn := (config.Database{Driver: "sqlite", Path: dbPath}).SQLiteDSN()
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	require.NoError(t, goose.SetDialect("sqlite3"))
	goose.SetBaseFS(migrations.FS("sqlite"))

	require.NoError(t, goose.UpTo(db, ".", 46))
	require.False(t, columnExists(t, db, "product_last_restocks", "last_discount_per_item"),
		"last_discount_per_item should not exist before 00047")
	require.False(t, columnExists(t, db, "product_restock_logs", "discount_per_item"),
		"discount_per_item should not exist before 00047")

	require.NoError(t, goose.Up(db, "."))
	require.True(t, columnExists(t, db, "product_last_restocks", "last_discount_per_item"),
		"last_discount_per_item added by 00047")
	require.True(t, columnExists(t, db, "product_restock_logs", "discount_per_item"),
		"discount_per_item added by 00047")
}

// 00049 adds the grosir tier table plus two sale_items columns, and BACKFILLS
// list_price_snapshot from unit_price_snapshot. The backfill is the part that
// matters: without it every pre-existing cart line would resolve tiers against a
// list price of 0. Also pins the min_qty >= 2 CHECK, which is what makes
// tier_min_qty = 0 a trustworthy "no grosir" flag.
func TestSQLite_UpgradeTo00049_PriceTiers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "upgrade49.sqlite")
	dsn := (config.Database{Driver: "sqlite", Path: dbPath}).SQLiteDSN()
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	require.NoError(t, goose.SetDialect("sqlite3"))
	goose.SetBaseFS(migrations.FS("sqlite"))

	require.NoError(t, goose.UpTo(db, ".", 48))
	require.False(t, tableExists(t, db, "product_price_tiers"), "table should not exist before 00049")
	require.False(t, columnExists(t, db, "sale_items", "list_price_snapshot"),
		"list_price_snapshot should not exist before 00049")
	require.False(t, columnExists(t, db, "sale_items", "tier_min_qty"),
		"tier_min_qty should not exist before 00049")

	// A legacy line priced before the feature existed. FKs are off so we can
	// insert it without seeding a whole sale graph.
	_, err = db.Exec("PRAGMA foreign_keys=OFF")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO sale_items (id, sale_id, product_id, qty, unit_price_snapshot, line_total)
	                  VALUES ('legacy-1', 'sale-1', 'prod-1', 3, 9000, 27000)`)
	require.NoError(t, err)

	require.NoError(t, goose.Up(db, "."))
	require.True(t, tableExists(t, db, "product_price_tiers"), "table added by 00049")
	require.True(t, columnExists(t, db, "sale_items", "list_price_snapshot"), "column added by 00049")
	require.True(t, columnExists(t, db, "sale_items", "tier_min_qty"), "column added by 00049")

	var listPrice, tierMinQty int64
	require.NoError(t, db.QueryRow(
		"SELECT list_price_snapshot, tier_min_qty FROM sale_items WHERE id = 'legacy-1'").
		Scan(&listPrice, &tierMinQty))
	require.Equal(t, int64(9000), listPrice, "backfilled from unit_price_snapshot")
	require.Equal(t, int64(0), tierMinQty, "legacy lines carry no grosir")

	// min_qty must be >= 2.
	_, err = db.Exec(`INSERT INTO product_price_tiers (id, product_id, product_unit_id, min_qty, price)
	                  VALUES ('t-1', 'prod-1', 'unit-1', 1, 8500)`)
	require.Error(t, err, "min_qty = 1 violates the CHECK")
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

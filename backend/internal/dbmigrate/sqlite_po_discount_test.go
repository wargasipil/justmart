package dbmigrate_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/go-sqlite" // pure-Go "sqlite" database/sql driver
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/migrations"
)

// TestSQLite_PODiscountBackfill reproduces the upgrade bug "table
// purchase_order_items has no column named discount_type": an existing SQLite DB
// that predates the PO discount columns. The current consolidated init already
// has them, so we simulate the historical gap by applying everything except the
// new v40 backfill, dropping the columns, then letting goose apply v40 — which
// must re-add them. Proves the migration runs via goose and is the real fix.
func TestSQLite_PODiscountBackfill(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "po-discount.sqlite")
	dsn := (config.Database{Driver: "sqlite", Path: dbPath}).SQLiteDSN()
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	migrations.SetActiveDriver("sqlite")
	require.NoError(t, goose.SetDialect("sqlite3"))
	goose.SetBaseFS(migrations.FS("sqlite"))

	// Apply everything EXCEPT the v40 backfill (the highest SQL migration is 39).
	require.NoError(t, goose.UpTo(db, ".", 39))

	// Simulate a pre-00034 DB: drop the discount columns the old baseline lacked.
	_, err = db.Exec(`ALTER TABLE purchase_order_items DROP COLUMN discount_value`)
	require.NoError(t, err)
	_, err = db.Exec(`ALTER TABLE purchase_order_items DROP COLUMN discount_type`)
	require.NoError(t, err)
	require.False(t, columnExists(t, db, "purchase_order_items", "discount_type"),
		"precondition: column dropped to simulate the old DB")

	// Apply v40 → backfill re-adds the missing columns.
	require.NoError(t, goose.Up(db, "."))

	require.True(t, columnExists(t, db, "purchase_order_items", "discount_type"),
		"discount_type re-added by 00040 backfill")
	require.True(t, columnExists(t, db, "purchase_order_items", "discount_value"))

	// The PO columns from 00027/00028 were untouched (still present, not duplicated).
	require.True(t, columnExists(t, db, "purchase_orders", "cart_discount"))
	require.True(t, columnExists(t, db, "purchase_orders", "ppn_rate"))
}

// TestBackfillPOColumns_AddsMissingAndIdempotent drives the helper directly
// against a table that lacks every target column, then re-runs to prove it's a
// no-op the second time (safe on fresh DBs that already have the columns).
func TestBackfillPOColumns_AddsMissingAndIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "backfill.sqlite")
	dsn := (config.Database{Driver: "sqlite", Path: dbPath}).SQLiteDSN()
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	// Minimal tables with NONE of the backfilled columns (old baseline shape).
	_, err = db.Exec(`CREATE TABLE purchase_orders (id TEXT PRIMARY KEY)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE purchase_order_items (id TEXT PRIMARY KEY)`)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, migrations.BackfillPOColumns(ctx, db))

	for _, c := range []struct{ table, col string }{
		{"purchase_orders", "invoice_no"},
		{"purchase_orders", "due_at"},
		{"purchase_orders", "subtotal"},
		{"purchase_orders", "cart_discount"},
		{"purchase_orders", "ppn_enabled"},
		{"purchase_orders", "ppn_amount"},
		{"purchase_orders", "ppn_rate"},
		{"purchase_order_items", "discount_type"},
		{"purchase_order_items", "discount_value"},
	} {
		require.Truef(t, columnExists(t, db, c.table, c.col), "%s.%s should be added", c.table, c.col)
	}

	// Idempotent: a second run on a now-complete schema is a clean no-op.
	require.NoError(t, migrations.BackfillPOColumns(ctx, db))

	// A row with discount_type is now insertable (FKs off — isolated column test).
	_, err = db.Exec(`INSERT INTO purchase_order_items (id, discount_type, discount_value) VALUES ('x','PERCENT',1000)`)
	require.NoError(t, err)
}

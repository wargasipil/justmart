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

// TestSQLite_SupplierAddressBankBackfill reproduces the upgrade bug "table
// suppliers has no column named address": an existing SQLite DB that predates
// the supplier address/bank columns (Postgres got them via 00033; SQLite only
// ever had them folded into the frozen consolidated init). The current init
// already has them, so we simulate the historical gap by applying everything
// except the new v52 backfill, dropping the columns, then letting goose apply
// v52 — which must re-add them. Proves the migration runs via goose.
func TestSQLite_SupplierAddressBankBackfill(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "supplier-address.sqlite")
	dsn := (config.Database{Driver: "sqlite", Path: dbPath}).SQLiteDSN()
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	migrations.SetActiveDriver("sqlite")
	require.NoError(t, goose.SetDialect("sqlite3"))
	goose.SetBaseFS(migrations.FS("sqlite"))

	// Apply everything EXCEPT the v52 backfill (the highest SQL migration is 51).
	require.NoError(t, goose.UpTo(db, ".", 51))

	// Simulate a pre-00033 DB: drop the columns the old baseline lacked.
	for _, col := range []string{"address", "bank_name", "bank_account_number", "bank_account_holder"} {
		_, err = db.Exec(`ALTER TABLE suppliers DROP COLUMN ` + col)
		require.NoError(t, err)
	}
	require.False(t, columnExists(t, db, "suppliers", "address"),
		"precondition: column dropped to simulate the old DB")

	// A supplier row from before the upgrade must survive the backfill.
	_, err = db.Exec(`INSERT INTO suppliers (id, code, name) VALUES ('s1','SUP-0001','Legacy Supplier')`)
	require.NoError(t, err)

	// Apply v52 → backfill re-adds the missing columns.
	require.NoError(t, goose.Up(db, "."))

	for _, col := range []string{"address", "bank_name", "bank_account_number", "bank_account_holder"} {
		require.Truef(t, columnExists(t, db, "suppliers", col), "%s re-added by 00052 backfill", col)
	}

	// Pre-existing data preserved, and the new columns defaulted (not NULL) so
	// the NOT NULL constraint holds for old rows too.
	var name, address string
	require.NoError(t, db.QueryRow(`SELECT name, address FROM suppliers WHERE id='s1'`).Scan(&name, &address))
	require.Equal(t, "Legacy Supplier", name)
	require.Equal(t, "", address)

	// The real regression: an INSERT naming every column now succeeds. Before
	// the backfill this failed with "no column named address" — which
	// CreateSupplier reported as "supplier.code_taken".
	_, err = db.Exec(`INSERT INTO suppliers (id, code, name, address, bank_name, bank_account_number, bank_account_holder)
		VALUES ('s2','SUP-0002','New Supplier','Jl. Mawar 1','BCA','123456','PT Sehat')`)
	require.NoError(t, err)
}

// TestBackfillSupplierColumns_AddsMissingAndIdempotent drives the helper
// directly against a table that lacks every target column, then re-runs to
// prove it's a no-op the second time (safe on fresh DBs that already have them).
func TestBackfillSupplierColumns_AddsMissingAndIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "supplier-backfill.sqlite")
	dsn := (config.Database{Driver: "sqlite", Path: dbPath}).SQLiteDSN()
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	// Minimal table with NONE of the backfilled columns (old baseline shape).
	_, err = db.Exec(`CREATE TABLE suppliers (id TEXT PRIMARY KEY, code TEXT NOT NULL, name TEXT NOT NULL)`)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, migrations.BackfillSupplierColumns(ctx, db))

	for _, col := range []string{"address", "bank_name", "bank_account_number", "bank_account_holder"} {
		require.Truef(t, columnExists(t, db, "suppliers", col), "suppliers.%s should be added", col)
	}

	// Idempotent: a second run on a now-complete schema is a clean no-op.
	require.NoError(t, migrations.BackfillSupplierColumns(ctx, db))

	_, err = db.Exec(`INSERT INTO suppliers (id, code, name, address) VALUES ('x','SUP-0009','X','Jl. Melati 2')`)
	require.NoError(t, err)
}

package dbmigrate_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/migrations"
)

// TestSQLite_UpgradeTo00065_ReceiptReplacedBy proves an existing SQLite DB
// (migrated to 00064) gains purchase_receipts.replaced_by_id in place, that a
// receipt written before the upgrade reads NULL ("not replaced"), and that a
// voided receipt can then point at the receipt that superseded it.
func TestSQLite_UpgradeTo00065_ReceiptReplacedBy(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "upgrade65.sqlite")
	dsn := (config.Database{Driver: "sqlite", Path: dbPath}).SQLiteDSN()
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	require.NoError(t, goose.SetDialect("sqlite3"))
	goose.SetBaseFS(migrations.FS("sqlite"))

	require.NoError(t, goose.UpTo(db, ".", 64))
	require.False(t, columnExists(t, db, "purchase_receipts", "replaced_by_id"))

	// A receipt from before the upgrade (FKs off: no full graph needed).
	_, err = db.Exec(`PRAGMA foreign_keys=OFF`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO purchase_receipts (id, receipt_no, purchase_order_id, received_at, received_by)
		VALUES ('r-old','RCV-2026-0001','po-1','2026-01-02','u-1')`)
	require.NoError(t, err)

	require.NoError(t, goose.Up(db, "."))
	require.True(t, columnExists(t, db, "purchase_receipts", "replaced_by_id"))

	var replacedBy sql.NullString
	require.NoError(t, db.QueryRow(
		`SELECT replaced_by_id FROM purchase_receipts WHERE id = 'r-old'`).Scan(&replacedBy))
	require.False(t, replacedBy.Valid, "a pre-00065 receipt was never replaced")

	// A new receipt supersedes it.
	_, err = db.Exec(`INSERT INTO purchase_receipts (id, receipt_no, purchase_order_id, received_at, received_by)
		VALUES ('r-new','RCV-2026-0002','po-1','2026-01-02','u-1')`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE purchase_receipts SET replaced_by_id = 'r-new' WHERE id = 'r-old'`)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(
		`SELECT replaced_by_id FROM purchase_receipts WHERE id = 'r-old'`).Scan(&replacedBy))
	require.Equal(t, "r-new", replacedBy.String)

	// Down drops the column again and keeps the receipts.
	require.NoError(t, goose.DownTo(db, ".", 64))
	require.False(t, columnExists(t, db, "purchase_receipts", "replaced_by_id"))
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM purchase_receipts`).Scan(&n))
	require.Equal(t, 2, n)
}

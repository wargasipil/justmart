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

// TestSQLite_UpgradeTo00064_ExactPackCost proves an existing SQLite DB
// (migrated to 00063) gains purchase_order_items.pack_cost_price and
// batches.cost_pack_price / cost_pack_factor in place, and that rows written
// before the upgrade read 0 -- the "legacy, derive from the per-base cost"
// value every reader of those columns falls back on.
func TestSQLite_UpgradeTo00064_ExactPackCost(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "upgrade64.sqlite")
	dsn := (config.Database{Driver: "sqlite", Path: dbPath}).SQLiteDSN()
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	require.NoError(t, goose.SetDialect("sqlite3"))
	goose.SetBaseFS(migrations.FS("sqlite"))

	require.NoError(t, goose.UpTo(db, ".", 63))
	require.False(t, columnExists(t, db, "purchase_order_items", "pack_cost_price"))
	require.False(t, columnExists(t, db, "batches", "cost_pack_price"))
	require.False(t, columnExists(t, db, "batches", "cost_pack_factor"))

	// A line and a lot from before the upgrade (FKs off: no full graph needed).
	_, err = db.Exec(`PRAGMA foreign_keys=OFF`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO purchase_order_items
		(id, purchase_order_id, product_id, ordered_qty, unit_cost_price, subtotal, unit_factor)
		VALUES ('poi-old','po-1','p-1',1500,2167,3250500,30)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO batches (id, product_id, expiry_date, cost_price)
		VALUES ('b-old','p-1','2030-01-31',2167)`)
	require.NoError(t, err)

	require.NoError(t, goose.Up(db, "."))
	require.True(t, columnExists(t, db, "purchase_order_items", "pack_cost_price"))
	require.True(t, columnExists(t, db, "batches", "cost_pack_price"))
	require.True(t, columnExists(t, db, "batches", "cost_pack_factor"))

	var pack int64
	require.NoError(t, db.QueryRow(
		`SELECT pack_cost_price FROM purchase_order_items WHERE id = 'poi-old'`).Scan(&pack))
	require.Zero(t, pack, "a pre-00064 line is a legacy line")
	var lotPack, lotFactor int64
	require.NoError(t, db.QueryRow(
		`SELECT cost_pack_price, cost_pack_factor FROM batches WHERE id = 'b-old'`).Scan(&lotPack, &lotFactor))
	require.Zero(t, lotPack)
	require.Zero(t, lotFactor)

	// And a new row can set them.
	_, err = db.Exec(`INSERT INTO batches (id, product_id, expiry_date, cost_price, cost_pack_price, cost_pack_factor)
		VALUES ('b-new','p-1','2030-01-31',2167,65000,30)`)
	require.NoError(t, err)
}

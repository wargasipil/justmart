package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

// 00040 repairs SQLite databases created from a consolidated 00001_init baseline
// that PREDATES the PO invoice/tax/discount columns. Those columns were added to
// Postgres via incrementals 00027/00028/00034 but on SQLite were only folded
// into the (frozen) consolidated init — with NO matching incremental. So a fresh
// SQLite DB has them, but an existing DB upgraded in place never gets them, and
// CreatePurchaseOrder fails: "table purchase_order_items has no column named
// discount_type". This migration adds any missing PO column, conditionally, so
// it's safe on every DB state: fresh SQLite (skips — already present), a partial
// pre-00034 DB (adds the discount pair), a pre-00027 DB (adds all), and Postgres
// (no-op — the columns already exist there via 00027/28/34). Plain SQL can't do
// this: SQLite has no ADD COLUMN IF NOT EXISTS and a fresh DB would error on a
// duplicate column, so it's a Go migration. Idempotent.
func init() {
	goose.AddNamedMigrationContext("00040_po_discount_backfill.go", up00040, down00040)
}

// poBackfillColumns lists the columns (DDL fragments mirroring the consolidated
// sqlite/00001_init.sql) that must exist on each table. Conditionally added.
var poBackfillColumns = []struct {
	table   string
	column  string
	ddl     string // the "<col> <type> ..." passed to ADD COLUMN
}{
	{"purchase_orders", "invoice_no", "invoice_no TEXT NOT NULL DEFAULT ''"},
	{"purchase_orders", "due_at", "due_at DATE"},
	{"purchase_orders", "subtotal", "subtotal INTEGER NOT NULL DEFAULT 0"},
	{"purchase_orders", "cart_discount", "cart_discount INTEGER NOT NULL DEFAULT 0"},
	{"purchase_orders", "ppn_enabled", "ppn_enabled INTEGER NOT NULL DEFAULT 0"},
	{"purchase_orders", "ppn_amount", "ppn_amount INTEGER NOT NULL DEFAULT 0"},
	{"purchase_orders", "ppn_rate", "ppn_rate INTEGER NOT NULL DEFAULT 11"},
	{"purchase_order_items", "discount_type", "discount_type TEXT NOT NULL DEFAULT 'FIXED' CHECK (discount_type IN ('FIXED','PERCENT'))"},
	{"purchase_order_items", "discount_value", "discount_value INTEGER NOT NULL DEFAULT 0"},
}

func up00040(ctx context.Context, tx *sql.Tx) error {
	// Only SQLite needs the repair; Postgres already has these columns (00027/
	// 28/34) and PRAGMA isn't valid there.
	if ActiveDriver() != "sqlite" {
		return nil
	}
	return BackfillPOColumns(ctx, tx)
}

func down00040(ctx context.Context, tx *sql.Tx) error {
	// Never drop the columns / their data on rollback.
	return nil
}

// BackfillPOColumns adds any missing PO invoice/tax/discount column to a SQLite
// DB. Idempotent: each ADD COLUMN is guarded by a PRAGMA table_info check, so
// running it on a DB that already has the columns is a no-op. Exported for the
// regression test. The querier is *sql.Tx in the migration; *sql.DB in tests.
func BackfillPOColumns(ctx context.Context, q sqlExecQuerier) error {
	for _, c := range poBackfillColumns {
		has, err := sqliteHasColumn(ctx, q, c.table, c.column)
		if err != nil {
			return err
		}
		if has {
			continue
		}
		if _, err := q.ExecContext(ctx, "ALTER TABLE "+c.table+" ADD COLUMN "+c.ddl); err != nil {
			return fmt.Errorf("backfill %s.%s: %w", c.table, c.column, err)
		}
	}
	return nil
}

// sqlExecQuerier is satisfied by both *sql.Tx and *sql.DB.
type sqlExecQuerier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// sqliteHasColumn reports whether `table` already has `column` (PRAGMA table_info).
func sqliteHasColumn(ctx context.Context, q sqlExecQuerier, table, column string) (bool, error) {
	rows, err := q.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return false, fmt.Errorf("pragma table_info(%s): %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

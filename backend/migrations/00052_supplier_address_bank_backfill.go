package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

// 00052 repairs SQLite databases created from a consolidated 00001_init baseline
// that PREDATES the supplier address + bank columns. Same failure shape as
// 00040: those columns reached Postgres via incremental 00033, but on SQLite
// were only folded into the (frozen) consolidated init with NO matching
// incremental. A fresh SQLite DB therefore has them while an existing DB
// upgraded in place never does, and CreateSupplier fails on INSERT — surfacing
// (before the error was un-masked) as a bogus "supplier.code_taken".
//
// Conditional, so it is safe on every DB state: fresh SQLite (skips — already
// present), an existing pre-00033 DB (adds all four), and Postgres (no-op — the
// columns exist there via 00033). Plain SQL can't express this: SQLite has no
// ADD COLUMN IF NOT EXISTS and a fresh DB would error on a duplicate column.
// Idempotent.
func init() {
	goose.AddNamedMigrationContext("00052_supplier_address_bank_backfill.go", up00052, down00052)
}

// supplierBackfillColumns mirrors the suppliers DDL in the consolidated
// sqlite/00001_init.sql. Conditionally added.
var supplierBackfillColumns = []struct {
	table  string
	column string
	ddl    string // the "<col> <type> ..." passed to ADD COLUMN
}{
	{"suppliers", "address", "address TEXT NOT NULL DEFAULT ''"},
	{"suppliers", "bank_name", "bank_name TEXT NOT NULL DEFAULT ''"},
	{"suppliers", "bank_account_number", "bank_account_number TEXT NOT NULL DEFAULT ''"},
	{"suppliers", "bank_account_holder", "bank_account_holder TEXT NOT NULL DEFAULT ''"},
}

func up00052(ctx context.Context, tx *sql.Tx) error {
	// Only SQLite needs the repair; Postgres already has these columns (00033)
	// and PRAGMA isn't valid there.
	if ActiveDriver() != "sqlite" {
		return nil
	}
	return BackfillSupplierColumns(ctx, tx)
}

func down00052(ctx context.Context, tx *sql.Tx) error {
	// Never drop the columns / their data on rollback.
	return nil
}

// BackfillSupplierColumns adds any missing supplier address/bank column to a
// SQLite DB. Idempotent: each ADD COLUMN is guarded by a PRAGMA table_info
// check, so running it on a DB that already has the columns is a no-op.
// Exported for the regression test. The querier is *sql.Tx in the migration;
// *sql.DB in tests.
func BackfillSupplierColumns(ctx context.Context, q sqlExecQuerier) error {
	for _, c := range supplierBackfillColumns {
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

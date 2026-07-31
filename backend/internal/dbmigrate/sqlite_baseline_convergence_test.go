package dbmigrate_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "github.com/glebarez/go-sqlite" // pure-Go "sqlite" database/sql driver
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/migrations"
)

// THE GUARD. sqlite/00001_init.sql is a FROZEN consolidated baseline: it is the
// version already applied in production, and goose never re-runs an applied
// version. So a column added to that file instead of to a new numbered
// migration reaches fresh databases only — every existing database silently
// lacks it, and the first INSERT naming that column fails in production.
//
// That has already happened twice, both in commit 2d0e784 ("pemasok"), which
// edited the frozen init to add:
//   - purchase_order_items.discount_type / discount_value  → repaired by 00040
//   - suppliers.address / bank_name / bank_account_number / bank_account_holder
//     → repaired by 00052
// The second went unnoticed for months because nothing checked. This test is
// that check.
//
// It pins the ORIGINAL released baseline (testdata/sqlite_init_v1.sql, the file
// as of commit 3a25e39 "add sqlite support") and asserts that a database built
// from it and then upgraded reaches EXACTLY the schema of a database built from
// scratch today. Any future edit to the frozen init without a matching
// incremental fails here, naming the drifted table and column.
//
// DO NOT "fix" a failure by regenerating the fixture: the fixture is what
// production actually ran. Add a new numbered migration (or a conditional Go
// migration, like 00040/00052, when SQLite's lack of ADD COLUMN IF NOT EXISTS
// means fresh DBs would collide).
func TestSQLite_FrozenBaselineConvergesWithFresh(t *testing.T) {
	legacy := migrateLegacyBaseline(t)
	defer legacy.Close()
	fresh := migrateFromScratch(t)
	defer fresh.Close()

	legacySchema := schemaOf(t, legacy)
	freshSchema := schemaOf(t, fresh)

	// Report every difference at once — a drifting commit usually adds several
	// columns, and fixing them one test-run at a time is miserable.
	var problems []string
	for _, table := range sortedKeys(freshSchema) {
		freshCols := freshSchema[table]
		legacyCols, ok := legacySchema[table]
		if !ok {
			problems = append(problems, fmt.Sprintf(
				"table %q exists on a fresh DB but NOT on an upgraded legacy DB", table))
			continue
		}
		for _, col := range freshCols {
			if !contains(legacyCols, col) {
				problems = append(problems, fmt.Sprintf(
					"%s.%s exists on a fresh DB but NOT on an upgraded legacy DB", table, col))
			}
		}
		for _, col := range legacyCols {
			if !contains(freshCols, col) {
				problems = append(problems, fmt.Sprintf(
					"%s.%s exists on an upgraded legacy DB but NOT on a fresh DB", table, col))
			}
		}
	}
	for _, table := range sortedKeys(legacySchema) {
		if _, ok := freshSchema[table]; !ok {
			problems = append(problems, fmt.Sprintf(
				"table %q exists on an upgraded legacy DB but NOT on a fresh DB", table))
		}
	}

	require.Emptyf(t, problems,
		"SQLite schema drift between an upgraded EXISTING database and a fresh one.\n"+
			"Something was added to the frozen sqlite/00001_init.sql without a matching\n"+
			"numbered migration, so only fresh databases get it — production does not.\n"+
			"Fix: add a new numbered migration (conditional Go migration if fresh DBs\n"+
			"would collide on an existing column — see 00040 / 00052). Do NOT edit the\n"+
			"frozen baseline or this fixture.\n\ndrift:\n  %s",
		strings.Join(problems, "\n  "))
}

// migrateLegacyBaseline builds the database an EXISTING deployment has: the
// original consolidated baseline, stamped as goose version 1, then every
// incremental applied on top — exactly what `migrate up` does on a live box.
func migrateLegacyBaseline(t *testing.T) *sql.DB {
	t.Helper()
	db := openSQLite(t, "legacy.sqlite")

	raw, err := os.ReadFile(filepath.Join("testdata", "sqlite_init_v1.sql"))
	require.NoError(t, err, "the pinned original baseline must stay checked in")

	// Execute only the Up half of the goose file.
	up := string(raw)
	if i := strings.Index(up, "-- +goose Down"); i >= 0 {
		up = up[:i]
	}
	_, err = db.Exec(up)
	require.NoError(t, err, "applying the pinned v1 baseline")

	// Stamp it as goose version 1 so goose applies only the incrementals — the
	// whole point being that it will NOT re-run the (now edited) init.
	_, err = db.Exec(`CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT (datetime('now')))`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO goose_db_version (version_id, is_applied) VALUES (0, 1), (1, 1)`)
	require.NoError(t, err)

	migrations.SetActiveDriver("sqlite")
	require.NoError(t, goose.SetDialect("sqlite3"))
	goose.SetBaseFS(migrations.FS("sqlite"))
	require.NoError(t, goose.Up(db, "."), "upgrading a legacy DB in place must succeed")
	return db
}

// migrateFromScratch builds the database a NEW deployment gets.
func migrateFromScratch(t *testing.T) *sql.DB {
	t.Helper()
	db := openSQLite(t, "fresh.sqlite")
	migrations.SetActiveDriver("sqlite")
	require.NoError(t, goose.SetDialect("sqlite3"))
	goose.SetBaseFS(migrations.FS("sqlite"))
	require.NoError(t, goose.Up(db, "."))
	return db
}

func openSQLite(t *testing.T, name string) *sql.DB {
	t.Helper()
	dsn := (config.Database{Driver: "sqlite", Path: filepath.Join(t.TempDir(), name)}).SQLiteDSN()
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	db.SetMaxOpenConns(1) // single-writer; the NO-TRANSACTION rebuilds need one conn
	return db
}

// schemaOf returns table name -> sorted column names, skipping goose's own
// bookkeeping and sqlite internals.
func schemaOf(t *testing.T, db *sql.DB) map[string][]string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table'
		AND name NOT LIKE 'sqlite_%' AND name <> 'goose_db_version' ORDER BY name`)
	require.NoError(t, err)
	var tables []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		tables = append(tables, n)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())

	out := make(map[string][]string, len(tables))
	for _, table := range tables {
		cols := []string{}
		cr, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
		require.NoError(t, err)
		for cr.Next() {
			var c string
			require.NoError(t, cr.Scan(&c))
			cols = append(cols, c)
		}
		require.NoError(t, cr.Err())
		require.NoError(t, cr.Close())
		sort.Strings(cols)
		out[table] = cols
	}
	return out
}

func sortedKeys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func contains(hay []string, needle string) bool {
	for _, s := range hay {
		if s == needle {
			return true
		}
	}
	return false
}

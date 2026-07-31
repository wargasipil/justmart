package backup_test

import (
	"os/exec"
	"testing"

	"github.com/justmart/backend/internal/config"
)

// The backup create/list tests are the ONLY co-located unit tests that are
// intrinsically engine-aware: on SQLite CreateBackup does VACUUM INTO ->
// database.sqlite; on Postgres it shells out to pg_dump -> database.sql.gz.
// These helpers let the same tests run under `make test-unit-postgres`.

// NO t.Parallel() ANYWHERE IN THIS PACKAGE — deliberate, and the one exception
// to the "t.Parallel() is safe, every test owns its own throwaway DB" rule.
//
// That rule holds because each Postgres test gets its own SCHEMA inside one
// shared database. pg_dump does not respect that boundary: it dumps the WHOLE
// database, enumerating every per-test schema. So when a parallel sibling drops
// its schema between pg_dump listing objects and querying them, the dump dies
// with `ERROR: schema "t_testxxx_123_4" does not exist` — a test failing because
// of an unrelated test's cleanup.
//
// It only bites where pg_dump actually runs: on a dev machine these tests SKIP
// (no pg_dump on PATH), so it stayed invisible until CI installed the client.
// Keep these tests sequential; `-p 1` already serialises packages.

// requirePGDumpOrSkip skips a backup test on Postgres when pg_dump isn't on PATH
// (mirrors e2e/backup_test.go). SQLite needs no external tooling, so it never
// skips there.
func requirePGDumpOrSkip(t *testing.T, cfg *config.Config) {
	t.Helper()
	if cfg.Database.IsSQLite() {
		return
	}
	if _, err := exec.LookPath("pg_dump"); err != nil {
		t.Skipf("pg_dump not on PATH (%v); skipping the Postgres backup test", err)
	}
}

// dumpFileName is the engine-specific dump filename CreateBackup writes.
func dumpFileName(cfg *config.Config) string {
	if cfg.Database.IsSQLite() {
		return "database.sqlite"
	}
	return "database.sql.gz"
}

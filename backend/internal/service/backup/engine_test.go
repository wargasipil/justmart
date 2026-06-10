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

package migrations

import (
	"embed"
	"io/fs"
)

// postgresFS / sqliteFS hold the per-engine goose migrations, embedded into the
// binary so the server (and the migrate command) run them without the source
// tree on disk. This is what makes the single self-contained binary work in
// Docker and on Windows.
//
// Postgres is the canonical, incrementally-evolved set (00001…). SQLite ships a
// single consolidated init that builds the current schema in SQLite dialect —
// new schema changes must be mirrored into BOTH sets going forward.
//
//go:embed postgres/*.sql
var postgresFS embed.FS

//go:embed sqlite/*.sql
var sqliteFS embed.FS

// FS returns the migration set for the given driver, rooted at the migrations
// themselves (goose runs against "."). driver is "postgres" or "sqlite".
func FS(driver string) fs.FS {
	sub := "postgres"
	if driver == "sqlite" {
		sub = "sqlite"
	}
	root, err := fs.Sub(engineFS(sub), sub)
	if err != nil {
		// Sub only errors on an invalid path; the dirs are compile-time embedded.
		panic(err)
	}
	return root
}

func engineFS(sub string) embed.FS {
	if sub == "sqlite" {
		return sqliteFS
	}
	return postgresFS
}

// activeDriver records the engine the current goose run is targeting. Go
// migrations (which only receive a *sql.Tx, not a dialect) read it to branch.
// Set by the boot path (dbmigrate.Run) and the CLI path (cmd/server migrate)
// before invoking goose. One process runs a single driver, so no locking.
var activeDriver string

// SetActiveDriver records the engine for the upcoming goose run ("postgres" or
// "sqlite"). Must be called before goose.Up / goose.RunContext.
func SetActiveDriver(driver string) { activeDriver = driver }

// ActiveDriver returns the driver set by SetActiveDriver (empty if unset).
func ActiveDriver() string { return activeDriver }

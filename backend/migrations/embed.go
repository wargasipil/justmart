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

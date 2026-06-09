// Package backup implements backup_iface.v1's BackupService — OWNER-only
// Create / List / Delete of per-timestamp pg_dump (Postgres) or VACUUM INTO
// (SQLite) archives. One RPC per file; shared helpers in helpers.go; the
// pg_dump binary resolver lives in pgdump.go.
package backup

import (
	"path/filepath"

	"gorm.io/gorm"

	"github.com/justmart/backend/internal/config"
)

// Backups writes per-timestamp pg_dump archives into a configured directory:
//
//	<cfg.Backup.Directory>/backup_<YYYY-mm-dd_HHMMSS>/
//	  database.sql.gz   (pg_dump --compress=6 output)
//	  manifest.txt      (created_at + db/schema versions + size_bytes)
//
// All RPCs are OWNER-only (enforced at the proto layer via allowed_roles).
//
// Restore is intentionally NOT exposed via RPC — it needs a maintenance-mode
// UX. CLI restore is documented in DEPLOYMENT.md.
type Backups struct {
	db        *gorm.DB
	cfg       *config.Config
	directory string // absolute or CWD-relative; created lazily on Create.
	// pgToolsDir is where resolvePgDump caches an auto-downloaded pg_dump
	// (Windows dev fallback). Empty disables caching/auto-download.
	pgToolsDir string
	// autoFetchPgDump enables the EDB binaries auto-download on Windows when
	// no pg_dump is found on PATH or bundled next to the justmart binary.
	// Production (NewBackupService) is true; tests (NewBackupServiceWithDir) are
	// false so a missing pg_dump skips cleanly without downloading 75 MB.
	autoFetchPgDump bool
}

// NewBackupService uses cfg.Backup.Directory + cfg.Backup.PgToolsDir and enables
// auto-fetch. Test code uses NewBackupServiceWithDir.
func NewBackupService(db *gorm.DB, cfg *config.Config) *Backups {
	return &Backups{
		db:              db,
		cfg:             cfg,
		directory:       cfg.Backup.Directory,
		pgToolsDir:      cfg.Backup.PgToolsDir,
		autoFetchPgDump: true,
	}
}

// NewBackupServiceWithDir is the test constructor — point it at t.TempDir().
// Tests keep autoFetchPgDump false so a missing pg_dump fails fast (via the
// existing LookPath skip guard) instead of triggering a 75 MB download.
func NewBackupServiceWithDir(db *gorm.DB, cfg *config.Config, directory string) *Backups {
	return &Backups{
		db:              db,
		cfg:             cfg,
		directory:       directory,
		pgToolsDir:      filepath.Join(directory, "_pgtools"),
		autoFetchPgDump: false,
	}
}

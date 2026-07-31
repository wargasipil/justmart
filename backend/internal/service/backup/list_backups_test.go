package backup_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	backupifacev1 "github.com/justmart/backend/gen/backup_iface/v1"
	backupsvc "github.com/justmart/backend/internal/service/backup"
	"github.com/justmart/backend/internal/service/servicetest"
)

// TestListBackups_NewestFirst creates two backups and asserts the listing
// returns both, newest-first.
func TestListBackups_NewestFirst(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	requirePGDumpOrSkip(t, cfg)
	svc := backupsvc.NewBackupServiceWithDir(gormDB, cfg, t.TempDir())

	first, err := svc.CreateBackup(context.Background(), connect.NewRequest(&backupifacev1.CreateBackupRequest{}))
	require.NoError(t, err)
	second, err := svc.CreateBackup(context.Background(), connect.NewRequest(&backupifacev1.CreateBackupRequest{}))
	require.NoError(t, err)

	resp, err := svc.ListBackups(context.Background(), connect.NewRequest(&backupifacev1.ListBackupsRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Backups, 2)

	// Newest first: CreatedAt descending. The second create has the later
	// (or equal-then-bumped) timestamp, so it sorts ahead of the first.
	require.GreaterOrEqual(t, resp.Msg.Backups[0].CreatedAt, resp.Msg.Backups[1].CreatedAt)

	names := map[string]bool{
		resp.Msg.Backups[0].Name: true,
		resp.Msg.Backups[1].Name: true,
	}
	require.True(t, names[first.Msg.Backup.Name])
	require.True(t, names[second.Msg.Backup.Name])

	// Sizes + schema version surface from the on-disk dump + manifest.
	require.Positive(t, resp.Msg.Backups[0].SizeBytes)
	require.Positive(t, resp.Msg.Backups[0].SchemaVersion)
	require.Equal(t, int32(2), resp.Msg.Total)
}

// The listing is a filesystem scan, so the page is sliced in Go — but the
// response still has to be bounded and `total` still has to be the full count.
func TestListBackups_Paginates(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	dir := t.TempDir()
	svc := backupsvc.NewBackupServiceWithDir(gormDB, cfg, dir)

	// Hand-build the directories rather than running CreateBackup 5x: this test
	// is about the slicing, and real backups need pg_dump on PATH.
	names := []string{
		"backup_2026-01-01_100000",
		"backup_2026-01-02_100000",
		"backup_2026-01-03_100000",
		"backup_2026-01-04_100000",
		"backup_2026-01-05_100000",
	}
	for _, n := range names {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, n), 0o755))
	}

	page := func(limit, offset int32) *backupifacev1.ListBackupsResponse {
		t.Helper()
		resp, err := svc.ListBackups(context.Background(), connect.NewRequest(
			&backupifacev1.ListBackupsRequest{Limit: limit, Offset: offset}))
		require.NoError(t, err)
		return resp.Msg
	}

	first := page(2, 0)
	require.Equal(t, int32(5), first.Total)
	require.Len(t, first.Backups, 2)
	// Newest first, so the Jan-05 dir leads.
	require.Equal(t, "backup_2026-01-05_100000", first.Backups[0].Name)

	seen := map[string]bool{}
	for off := int32(0); off < first.Total; off += 2 {
		pg := page(2, off)
		require.Equal(t, first.Total, pg.Total)
		for _, b := range pg.Backups {
			require.False(t, seen[b.Name], "backup %s appeared on two pages", b.Name)
			seen[b.Name] = true
		}
	}
	require.Len(t, seen, 5)

	// Offset past the end: no rows, real total (guards the slice bounds).
	last := page(2, 99)
	require.Empty(t, last.Backups)
	require.Equal(t, int32(5), last.Total)
}

// TestListBackups_MissingDirIsEmpty proves a never-created backup root returns
// an empty list, NOT an error (errors.Is(os.ErrNotExist) is swallowed).
func TestListBackups_MissingDirIsEmpty(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	// Point at a path that does not exist yet (no CreateBackup ever ran, so
	// ensureDir never created it).
	missing := filepath.Join(t.TempDir(), "never-created")
	svc := backupsvc.NewBackupServiceWithDir(gormDB, cfg, missing)

	resp, err := svc.ListBackups(context.Background(), connect.NewRequest(&backupifacev1.ListBackupsRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Backups)
}

// TestListBackups_IgnoresForeignEntries proves the regex filter: files and
// foreign directories under the backup root are silently skipped.
func TestListBackups_IgnoresForeignEntries(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	requirePGDumpOrSkip(t, cfg)
	dir := t.TempDir()
	svc := backupsvc.NewBackupServiceWithDir(gormDB, cfg, dir)

	created, err := svc.CreateBackup(context.Background(), connect.NewRequest(&backupifacev1.CreateBackupRequest{}))
	require.NoError(t, err)

	// Drop a stray file + a foreign directory next to the real backup.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.txt"), []byte("x"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "not-a-backup"), 0o755))

	resp, err := svc.ListBackups(context.Background(), connect.NewRequest(&backupifacev1.ListBackupsRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Backups, 1, "only the regex-matching backup dir should be listed")
	require.Equal(t, created.Msg.Backup.Name, resp.Msg.Backups[0].Name)
}

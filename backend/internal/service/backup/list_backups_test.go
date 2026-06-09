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

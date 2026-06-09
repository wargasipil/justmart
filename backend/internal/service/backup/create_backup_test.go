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

// TestCreateBackup_RoundTrip exercises the SQLite happy path: VACUUM INTO a
// fresh per-timestamp directory, then assert the response + on-disk layout
// (database.sqlite + manifest.txt) the listing relies on.
func TestCreateBackup_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	dir := t.TempDir()
	svc := backupsvc.NewBackupServiceWithDir(gormDB, cfg, dir)

	resp, err := svc.CreateBackup(context.Background(), connect.NewRequest(&backupifacev1.CreateBackupRequest{}))
	require.NoError(t, err)

	b := resp.Msg.Backup
	require.NotNil(t, b)
	require.Regexp(t, `^backup_\d{4}-\d{2}-\d{2}_\d{6}$`, b.Name)
	require.Positive(t, b.CreatedAt)
	require.Positive(t, b.SizeBytes, "the SQLite VACUUM INTO snapshot should be non-empty")
	require.Positive(t, b.SchemaVersion, "schema_version reads the max applied goose migration")

	// On-disk layout: the VACUUM INTO snapshot + the manifest.
	backupDir := filepath.Join(dir, b.Name)
	dump := filepath.Join(backupDir, "database.sqlite")
	info, statErr := os.Stat(dump)
	require.NoError(t, statErr, "database.sqlite must exist in the backup dir")
	require.Equal(t, info.Size(), b.SizeBytes)

	_, manifestErr := os.Stat(filepath.Join(backupDir, "manifest.txt"))
	require.NoError(t, manifestErr, "manifest.txt must be written")
}

// TestCreateBackup_TwoInSameSecond proves the per-second collision bump: two
// back-to-back creates land in two distinct directories (the handler advances
// the timestamp until the dir name is free, since VACUUM INTO refuses to
// overwrite an existing file).
func TestCreateBackup_TwoInSameSecond(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	svc := backupsvc.NewBackupServiceWithDir(gormDB, cfg, t.TempDir())

	first, err := svc.CreateBackup(context.Background(), connect.NewRequest(&backupifacev1.CreateBackupRequest{}))
	require.NoError(t, err)
	second, err := svc.CreateBackup(context.Background(), connect.NewRequest(&backupifacev1.CreateBackupRequest{}))
	require.NoError(t, err)

	require.NotEqual(t, first.Msg.Backup.Name, second.Msg.Backup.Name,
		"two backups must not collide on the same per-second directory name")
}

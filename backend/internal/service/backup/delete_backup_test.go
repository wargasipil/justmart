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

// TestDeleteBackup_RoundTrip creates a backup then deletes it, asserting the
// directory is gone and the listing no longer shows it.
func TestDeleteBackup_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	dir := t.TempDir()
	svc := backupsvc.NewBackupServiceWithDir(gormDB, cfg, dir)

	created, err := svc.CreateBackup(context.Background(), connect.NewRequest(&backupifacev1.CreateBackupRequest{}))
	require.NoError(t, err)
	name := created.Msg.Backup.Name

	_, err = svc.DeleteBackup(context.Background(), connect.NewRequest(&backupifacev1.DeleteBackupRequest{
		Name: name,
	}))
	require.NoError(t, err)

	// Directory is gone from disk...
	_, statErr := os.Stat(filepath.Join(dir, name))
	require.True(t, os.IsNotExist(statErr), "deleted backup dir must no longer exist")

	// ...and from the listing.
	resp, err := svc.ListBackups(context.Background(), connect.NewRequest(&backupifacev1.ListBackupsRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Backups)
}

// TestDeleteBackup_NotFound deletes a well-formed but nonexistent backup name —
// expect CodeNotFound.
func TestDeleteBackup_NotFound(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	svc := backupsvc.NewBackupServiceWithDir(gormDB, cfg, t.TempDir())

	_, err := svc.DeleteBackup(context.Background(), connect.NewRequest(&backupifacev1.DeleteBackupRequest{
		Name: "backup_2099-01-01_000000", // valid shape, never created
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestDeleteBackup_InvalidName proves the regex guard rejects path-traversal /
// arbitrary names with CodeInvalidArgument before any filesystem op.
func TestDeleteBackup_InvalidName(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	svc := backupsvc.NewBackupServiceWithDir(gormDB, cfg, t.TempDir())

	for _, bad := range []string{
		"",
		"../etc/passwd",
		"backup_2026-05-26_152400/../..",
		"random-dir",
	} {
		_, err := svc.DeleteBackup(context.Background(), connect.NewRequest(&backupifacev1.DeleteBackupRequest{
			Name: bad,
		}))
		require.Error(t, err, "name %q must be rejected", bad)
		require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "name %q", bad)
	}
}

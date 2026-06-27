package update

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStageRevert_StagesBackup(t *testing.T) {
	t.Parallel()
	exeDir := t.TempDir()
	bakBytes := []byte("OLD-BINARY-v1.2.0")
	require.NoError(t, os.WriteFile(filepath.Join(exeDir, BackupName), bakBytes, 0o644))

	require.True(t, HasBackup(exeDir))
	require.NoError(t, StageRevert(exeDir))

	// The backup was staged as the launcher's swap target...
	got, err := os.ReadFile(filepath.Join(exeDir, StagedName))
	require.NoError(t, err)
	require.Equal(t, bakBytes, got)
	// ...and the backup itself survives (copy, not move).
	_, err = os.Stat(filepath.Join(exeDir, BackupName))
	require.NoError(t, err)
	// No leftover temp file.
	_, err = os.Stat(filepath.Join(exeDir, StagedName+".part"))
	require.True(t, os.IsNotExist(err))
}

func TestStageRevert_NoBackup(t *testing.T) {
	t.Parallel()
	exeDir := t.TempDir()

	require.False(t, HasBackup(exeDir))
	err := StageRevert(exeDir)
	require.ErrorIs(t, err, ErrNoBackup)
	// Nothing staged.
	_, statErr := os.Stat(filepath.Join(exeDir, StagedName))
	require.True(t, os.IsNotExist(statErr))
}

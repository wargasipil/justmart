package update

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// BackupName is the filename the portable launcher (packaging/windows/
// portable-template/Start Justmart.bat) moves the OLD justmart.exe to when it
// applies a staged update — i.e. the previous version, kept for rollback. Revert
// re-stages it as StagedName so the SAME launcher swap rolls back on next start.
const BackupName = "justmart.exe.bak"

// ErrNoBackup is returned by StageRevert when there's no previous-version backup
// next to the binary (nothing to roll back to).
var ErrNoBackup = errors.New("no previous version to revert to")

// HasBackup reports whether a previous-version backup (BackupName) sits next to
// the running binary — i.e. whether a revert is possible.
func HasBackup(exeDir string) bool {
	_, err := os.Stat(filepath.Join(exeDir, BackupName))
	return err == nil
}

// StageRevert stages the previous-version backup as the update the launcher
// applies on next start: it copies <exeDir>/justmart.exe.bak over
// <exeDir>/justmart.exe.new. It does NOT swap or restart — the portable launcher
// swaps StagedName in on the next launch (the same path a forward update uses),
// which also moves the current exe to BackupName, leaving a fresh backup of the
// version we reverted FROM (so the user can toggle back). Returns ErrNoBackup
// when no backup exists. Serialized with Apply via applyMu.
func StageRevert(exeDir string) error {
	applyMu.Lock()
	defer applyMu.Unlock()

	bak := filepath.Join(exeDir, BackupName)
	if _, err := os.Stat(bak); err != nil {
		if os.IsNotExist(err) {
			return ErrNoBackup
		}
		return err
	}
	// Copy (not move) so the backup survives until the launcher swap — keeps
	// HasBackup true if the user never restarts, and the launcher recreates
	// BackupName from the current exe anyway.
	return copyFile(bak, filepath.Join(exeDir, StagedName))
}

// copyFile copies src to dst atomically (write to dst+".part", then rename),
// mirroring downloadFile/extractExe so a crash mid-copy never leaves a partial
// staged exe.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create staged file (is the folder writable?): %w", err)
	}
	if _, err := io.Copy(out, in); err != nil { //nolint:gosec // size bounded by our own binary
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

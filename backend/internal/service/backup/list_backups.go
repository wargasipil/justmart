package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"

	backupifacev1 "github.com/justmart/backend/gen/backup_iface/v1"
)

// ListBackups returns every backup_<timestamp>/ directory under the configured
// backup root, newest first. Non-matching entries (files, foreign dirs) are
// silently ignored.
func (s *Backups) ListBackups(
	_ context.Context,
	_ *connect.Request[backupifacev1.ListBackupsRequest],
) (*connect.Response[backupifacev1.ListBackupsResponse], error) {
	out := &backupifacev1.ListBackupsResponse{}
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// No backups have ever been taken — empty list, not an error.
			return connect.NewResponse(out), nil
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, e := range entries {
		if !e.IsDir() || !backupNameRe.MatchString(e.Name()) {
			continue
		}
		dir := filepath.Join(s.directory, e.Name())
		// The dump file is engine-specific (.sql.gz for Postgres, .sqlite for
		// SQLite); stat whichever is present so size is right for both.
		var size int64
		if info, serr := os.Stat(filepath.Join(dir, dumpFileName)); serr == nil {
			size = info.Size()
		} else if info, serr := os.Stat(filepath.Join(dir, sqliteDumpFileName)); serr == nil {
			size = info.Size()
		}
		// Parse the timestamp out of the name; fall back to dir mtime so a
		// listing never silently drops a real backup.
		var created int64
		if t, perr := time.ParseInLocation("2006-01-02_150405",
			strings.TrimPrefix(e.Name(), "backup_"), time.Local); perr == nil {
			created = t.Unix()
		} else if di, derr := e.Info(); derr == nil {
			created = di.ModTime().Unix()
		}
		schemaVersion := readManifestSchemaVersion(filepath.Join(dir, manifestFileName))
		out.Backups = append(out.Backups, &backupifacev1.Backup{
			Name:          e.Name(),
			CreatedAt:     created,
			SizeBytes:     size,
			SchemaVersion: schemaVersion,
		})
	}
	sort.Slice(out.Backups, func(i, j int) bool {
		return out.Backups[i].CreatedAt > out.Backups[j].CreatedAt
	})
	return connect.NewResponse(out), nil
}

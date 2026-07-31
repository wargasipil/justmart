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
	"github.com/justmart/backend/internal/service/common"
)

// ListBackups returns one page of the backup_<timestamp>/ directories under the
// configured backup root, newest first. Non-matching entries (files, foreign
// dirs) are silently ignored.
//
// The source is a filesystem scan, not SQL, so the page is sliced in Go after
// the full scan + sort. That still bounds the RESPONSE, which is the point — a
// shop that has been backing up nightly for two years has ~700 directories.
func (s *Backups) ListBackups(
	_ context.Context,
	req *connect.Request[backupifacev1.ListBackupsRequest],
) (*connect.Response[backupifacev1.ListBackupsResponse], error) {
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
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
	// Name is the tiebreak: two backups taken in the same second would otherwise
	// order non-deterministically and could repeat/vanish across pages. The name
	// carries the timestamp, so it's a stable, meaningful secondary key.
	sort.Slice(out.Backups, func(i, j int) bool {
		if out.Backups[i].CreatedAt != out.Backups[j].CreatedAt {
			return out.Backups[i].CreatedAt > out.Backups[j].CreatedAt
		}
		return out.Backups[i].Name > out.Backups[j].Name
	})
	out.Total = int32(len(out.Backups))
	if offset >= len(out.Backups) {
		out.Backups = nil
	} else {
		out.Backups = out.Backups[offset:min(offset+limit, len(out.Backups))]
	}
	return connect.NewResponse(out), nil
}

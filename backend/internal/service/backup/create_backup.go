package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	backupifacev1 "github.com/justmart/backend/gen/backup_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// CreateBackup runs pg_dump (compressed) into a fresh backup_<ts>/ directory.
// On any failure the partial directory is removed so the listing never shows
// a half-baked backup.
func (s *Backups) CreateBackup(
	ctx context.Context,
	_ *connect.Request[backupifacev1.CreateBackupRequest],
) (*connect.Response[backupifacev1.CreateBackupResponse], error) {
	if err := s.ensureDir(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Find a free per-second directory name. The name has 1-second resolution, so
	// two backups in the same second would collide — bump the timestamp until the
	// dir doesn't exist (matters for the fast SQLite path: VACUUM INTO refuses to
	// overwrite an existing file).
	now := time.Now()
	var name, dir string
	for {
		name = "backup_" + now.Format("2006-01-02_150405")
		dir = filepath.Join(s.directory, name)
		if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			break
		}
		now = now.Add(time.Second)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("mkdir backup: %w", err))
	}
	// Belt + suspenders: if anything below fails, drop the partial dir.
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(dir)
		}
	}()

	// Produce the dump. SQLite: a consistent online snapshot via VACUUM INTO a
	// fresh .sqlite file (no external tooling). Postgres: pg_dump (.sql.gz).
	var dumpPath string
	if common.IsSQLite(s.db) {
		dumpPath = filepath.Join(dir, sqliteDumpFileName)
		if err := s.db.WithContext(ctx).Exec("VACUUM INTO ?", dumpPath).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("sqlite backup: %w", err))
		}
	} else {
		// Resolve pg_dump: PATH → bundled-next-to-justmart-binary → cached download →
		// (Windows + autoFetch) auto-download EDB binaries → friendly error.
		pgDumpPath, err := resolvePgDump(ctx, s.pgToolsDir, s.autoFetchPgDump)
		if err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		dumpPath = filepath.Join(dir, dumpFileName)
		db := s.cfg.Database
		cmd := exec.CommandContext(ctx, pgDumpPath,
			"--host="+db.Host,
			"--port="+strconv.Itoa(db.Port),
			"--username="+db.User,
			"--dbname="+db.Name,
			"--no-password",
			"--clean",
			"--if-exists",
			"--compress=6",
			"--file="+dumpPath,
		)
		cmd.Env = append(os.Environ(), "PGPASSWORD="+db.Password)
		stderr := &strings.Builder{}
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("pg_dump: %s", msg))
		}
	}

	info, err := os.Stat(dumpPath)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("stat dump: %w", err))
	}
	size := info.Size()

	schemaVersion := s.readSchemaVersion(ctx)
	dbVersion := s.readDBVersion(ctx)

	if err := writeManifest(filepath.Join(dir, manifestFileName), manifest{
		CreatedAt:     now.Unix(),
		AppVersion:    appVersion,
		DBVersion:     dbVersion,
		SchemaVersion: schemaVersion,
		SizeBytes:     size,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("write manifest: %w", err))
	}

	committed = true
	return connect.NewResponse(&backupifacev1.CreateBackupResponse{
		Backup: &backupifacev1.Backup{
			Name:          name,
			CreatedAt:     now.Unix(),
			SizeBytes:     size,
			SchemaVersion: schemaVersion,
		},
	}), nil
}

package backup

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/justmart/backend/internal/service/common"
)

// backupNameRe matches a per-timestamp directory name. The same regex
// validates Delete's `name` arg, refusing `..`, slashes, anything else.
var backupNameRe = regexp.MustCompile(`^backup_\d{4}-\d{2}-\d{2}_\d{6}$`)

const (
	dumpFileName       = "database.sql.gz" // Postgres: pg_dump --compress=6 output
	sqliteDumpFileName = "database.sqlite" // SQLite: VACUUM INTO snapshot
	manifestFileName   = "manifest.txt"
)

// appVersion is a compile-time constant for the manifest. Wire a real value
// here later (ldflags); "dev" is fine for now.
const appVersion = "dev"

func (s *Backups) ensureDir() error {
	return os.MkdirAll(s.directory, 0o755)
}

func (s *Backups) readSchemaVersion(ctx context.Context) int32 {
	var v int64
	// goose writes (version_id, is_applied) rows; the max applied id is the
	// current schema version. Don't fail backup if the table is missing.
	err := s.db.WithContext(ctx).Raw(
		`SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied = true`,
	).Scan(&v).Error
	if err != nil {
		return 0
	}
	return int32(v)
}

func (s *Backups) readDBVersion(ctx context.Context) string {
	var v string
	q := `SELECT version()`
	if common.IsSQLite(s.db) {
		q = `SELECT sqlite_version()`
	}
	if err := s.db.WithContext(ctx).Raw(q).Scan(&v).Error; err != nil {
		return ""
	}
	return v
}

// manifest is the on-disk shape of manifest.txt. Plain key=value lines so
// `cat manifest.txt` works without any tooling.
type manifest struct {
	CreatedAt     int64
	AppVersion    string
	DBVersion     string
	SchemaVersion int32
	SizeBytes     int64
}

func writeManifest(path string, m manifest) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "created_at=%d\n", m.CreatedAt)
	fmt.Fprintf(w, "created_at_iso=%s\n", time.Unix(m.CreatedAt, 0).Format(time.RFC3339))
	fmt.Fprintf(w, "app_version=%s\n", m.AppVersion)
	fmt.Fprintf(w, "db_version=%s\n", m.DBVersion)
	fmt.Fprintf(w, "schema_version=%d\n", m.SchemaVersion)
	fmt.Fprintf(w, "size_bytes=%d\n", m.SizeBytes)
	return w.Flush()
}

// readManifestSchemaVersion best-effort parses schema_version out of an
// existing manifest. Returns 0 if the file is missing or unparseable — the
// listing still shows the row.
func readManifestSchemaVersion(path string) int32 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "schema_version=") {
			continue
		}
		n, perr := strconv.ParseInt(strings.TrimPrefix(line, "schema_version="), 10, 32)
		if perr != nil {
			return 0
		}
		return int32(n)
	}
	return 0
}

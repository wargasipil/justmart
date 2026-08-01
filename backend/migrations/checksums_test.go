package migrations_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// A committed migration has already run in production. goose records the
// version and never re-runs it, so editing the file changes what NEW databases
// get while every EXISTING database keeps the old shape — silently, until an
// INSERT hits the column that only exists on fresh installs.
//
// checksums.txt pins every migration's content. This test fails if a pinned
// file's bytes change, or if a migration is added without being pinned.
//
// Real incident: commit 2d0e784 edited the frozen sqlite/00001_init.sql to add
// purchase_order_items.discount_* and suppliers.address/bank_* with no matching
// incremental. The first needed repair migration 00040, the second 00052, and
// the second went unnoticed for months. Both would have failed here immediately.
//
// To ADD a migration: run `make migrate-checksums` (appends new files only).
// To CHANGE a migration: don't. Write a new numbered migration instead. If the
// change must be conditional because fresh DBs already have the column (SQLite
// has no ADD COLUMN IF NOT EXISTS), use a Go migration — see 00040 / 00052.
func TestMigrationsAreImmutable(t *testing.T) {
	pinned, err := readChecksums("checksums.txt")
	require.NoError(t, err, "checksums.txt must stay checked in")

	actual, err := hashMigrations(".")
	require.NoError(t, err)

	var changed, unpinned, missing []string
	for name, wantSum := range pinned {
		gotSum, ok := actual[name]
		if !ok {
			missing = append(missing, name)
			continue
		}
		if gotSum != wantSum {
			changed = append(changed, fmt.Sprintf("%s\n      pinned: %s\n      actual: %s", name, wantSum, gotSum))
		}
	}
	for name, sum := range actual {
		if _, ok := pinned[name]; !ok {
			unpinned = append(unpinned, fmt.Sprintf("%s  %s", sum, name))
		}
	}
	sort.Strings(changed)
	sort.Strings(unpinned)
	sort.Strings(missing)

	require.Emptyf(t, changed,
		"A migration that has ALREADY RUN in production was modified.\n"+
			"goose will not re-run it, so existing databases keep the old schema while\n"+
			"fresh ones get the new — the exact failure that produced migrations 00040\n"+
			"and 00052. Revert the edit and add a NEW numbered migration instead.\n\nchanged:\n    %s",
		strings.Join(changed, "\n    "))

	require.Emptyf(t, missing,
		"A pinned migration file is gone. Deleting an applied migration desynchronises\n"+
			"every existing database from the migration history. Restore it.\n\nmissing:\n    %s",
		strings.Join(missing, "\n    "))

	require.Emptyf(t, unpinned,
		"New migration(s) are not pinned in checksums.txt.\n"+
			"Run `make migrate-checksums` (it only appends) and commit the result.\n\nunpinned:\n    %s",
		strings.Join(unpinned, "\n    "))
}

// readChecksums parses "<sha256>  <path>" lines, ignoring blanks and comments.
func readChecksums(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("malformed checksums.txt line: %q", line)
		}
		out[filepath.ToSlash(fields[1])] = fields[0]
	}
	return out, nil
}

// hashMigrations hashes every migration: the per-engine .sql sets plus the
// numbered Go migrations in this directory. Content is normalised to LF so a
// checkout with CRLF line endings (Windows) hashes the same as CI.
func hashMigrations(root string) (map[string]string, error) {
	out := map[string]string{}
	add := func(path string) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		normalised := strings.ReplaceAll(string(raw), "\r\n", "\n")
		sum := sha256.Sum256([]byte(normalised))
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	}

	for _, engine := range []string{"postgres", "sqlite"} {
		entries, err := os.ReadDir(filepath.Join(root, engine))
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
				continue
			}
			if err := add(filepath.Join(root, engine, e.Name())); err != nil {
				return nil, err
			}
		}
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		// Numbered Go migrations only — embed.go and this test are not migrations.
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || !isDigit(e.Name()[0]) {
			continue
		}
		if err := add(filepath.Join(root, e.Name())); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

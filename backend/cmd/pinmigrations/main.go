// Command pinmigrations appends newly added migrations to
// backend/migrations/checksums.txt.
//
// APPEND-ONLY by design. An entry already in the manifest describes a migration
// that has run in production; if its content no longer matches, that is the bug
// TestMigrationsAreImmutable exists to catch, and rewriting the hash would hide
// it. This command therefore refuses to touch existing entries and reports the
// mismatch instead.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const manifest = "migrations/checksums.txt"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "pinmigrations:", err)
		os.Exit(1)
	}
}

func run() error {
	root := "migrations"
	existing, order, header, err := readManifest(manifest)
	if err != nil {
		return err
	}
	actual, err := hashAll(root)
	if err != nil {
		return err
	}

	var added, drifted []string
	for name, sum := range actual {
		prev, ok := existing[name]
		if !ok {
			existing[name] = sum
			order = append(order, name)
			added = append(added, name)
			continue
		}
		if prev != sum {
			drifted = append(drifted, name)
		}
	}
	sort.Strings(added)
	sort.Strings(drifted)

	if len(drifted) > 0 {
		return fmt.Errorf("these migrations have ALREADY RUN in production but their content changed:\n  %s\n"+
			"Revert the edit and add a NEW numbered migration instead (see 00040 / 00052 for the\n"+
			"conditional-Go-migration pattern when fresh DBs already have the column).",
			strings.Join(drifted, "\n  "))
	}
	if len(added) == 0 {
		fmt.Println("pinmigrations: nothing new to pin")
		return nil
	}

	var b strings.Builder
	b.WriteString(header)
	for _, name := range order {
		fmt.Fprintf(&b, "%s  %s\n", existing[name], name)
	}
	if err := os.WriteFile(manifest, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("pinmigrations: pinned %d new migration(s):\n  %s\n", len(added), strings.Join(added, "\n  "))
	return nil
}

// readManifest returns the pinned hashes, the order they appear in, and the
// leading comment block (preserved verbatim on rewrite).
func readManifest(path string) (map[string]string, []string, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, "", err
	}
	sums := map[string]string{}
	var order []string
	var header strings.Builder
	inHeader := true
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if inHeader && (trimmed == "" || strings.HasPrefix(trimmed, "#")) {
			header.WriteString(strings.TrimRight(line, "\r") + "\n")
			continue
		}
		if trimmed == "" {
			continue
		}
		inHeader = false
		fields := strings.Fields(trimmed)
		if len(fields) != 2 {
			return nil, nil, "", fmt.Errorf("malformed checksums.txt line: %q", trimmed)
		}
		name := filepath.ToSlash(fields[1])
		sums[name] = fields[0]
		order = append(order, name)
	}
	return sums, order, header.String(), nil
}

// hashAll mirrors TestMigrationsAreImmutable: per-engine .sql sets plus the
// numbered Go migrations, hashed with LF-normalised content.
func hashAll(root string) (map[string]string, error) {
	out := map[string]string{}
	add := func(path string) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256([]byte(strings.ReplaceAll(string(raw), "\r\n", "\n")))
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
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
				if err := add(filepath.Join(root, engine, e.Name())); err != nil {
					return nil, err
				}
			}
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && e.Name()[0] >= '0' && e.Name()[0] <= '9' {
			if err := add(filepath.Join(root, e.Name())); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

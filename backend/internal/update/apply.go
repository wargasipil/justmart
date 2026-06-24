package update

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// StagedName is the filename a downloaded update is staged under, next to the
// running binary. The portable launcher swaps it over justmart.exe on next start
// (Windows can't overwrite a running exe). Exported so the caller can report it.
const StagedName = "justmart.exe.new"

// applyMu serializes Apply so two concurrent "Update now" clicks can't download
// into / clobber the same staging files. In-process is enough (single-node).
var applyMu sync.Mutex

// Apply downloads info.AssetURL (the portable zip), verifies it against
// info.ChecksumURL (sha256), extracts justmart.exe, and stages it as
// <exeDir>/justmart.exe.new. It does NOT swap or restart — the launcher applies
// it on next start. Returns the staged version (info.Latest). OS-agnostic by
// design (the handler gates to Windows); a unit test drives it with a temp dir.
func Apply(ctx context.Context, client *http.Client, info Info, exeDir string) (string, error) {
	if info.AssetURL == "" || info.ChecksumURL == "" {
		return "", errors.New("release has no portable download + checksum to apply")
	}
	applyMu.Lock()
	defer applyMu.Unlock()

	work := filepath.Join(exeDir, ".update")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return "", fmt.Errorf("create update dir (is the folder writable?): %w", err)
	}
	defer os.RemoveAll(work)

	zipPath := filepath.Join(work, "justmart.zip")
	if err := downloadFile(ctx, client, info.AssetURL, zipPath); err != nil {
		return "", fmt.Errorf("download update: %w", err)
	}
	sumPath := filepath.Join(work, "justmart.zip.sha256")
	if err := downloadFile(ctx, client, info.ChecksumURL, sumPath); err != nil {
		return "", fmt.Errorf("download checksum: %w", err)
	}

	if err := verifySHA256(zipPath, sumPath); err != nil {
		return "", err // checksum mismatch / unreadable — refuse to stage
	}

	staged := filepath.Join(exeDir, StagedName)
	if err := extractExe(zipPath, staged); err != nil {
		return "", err
	}
	return info.Latest, nil
}

// downloadFile GETs url into dest (atomically via a .part temp), failing on a
// non-200. The caller's http.Client should carry a timeout.
func downloadFile(ctx context.Context, client *http.Client, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "justmart-updater")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

// verifySHA256 hashes zipPath and compares it to the hex digest in sumPath (the
// file holds the lowercase hex, optionally followed by a filename — we take the
// first token). A mismatch returns an error so a corrupt/tampered download is
// never staged.
func verifySHA256(zipPath, sumPath string) error {
	raw, err := os.ReadFile(sumPath)
	if err != nil {
		return err
	}
	want := strings.ToLower(strings.TrimSpace(string(raw)))
	if i := strings.IndexAny(want, " \t"); i >= 0 {
		want = want[:i]
	}
	if len(want) != 64 {
		return fmt.Errorf("malformed checksum (%d chars)", len(want))
	}

	f, err := os.Open(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, want)
	}
	return nil
}

// extractExe writes the zip's justmart.exe entry to dest (atomically). The zip
// layout is justmart-portable-<ver>/justmart.exe. We match by entry name and
// write to a fixed dest (no entry-derived path → no zip-slip risk).
func extractExe(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open update zip: %w", err)
	}
	defer r.Close()

	for _, zf := range r.File {
		name := strings.ToLower(strings.ReplaceAll(zf.Name, "\\", "/"))
		if name == "justmart.exe" || strings.HasSuffix(name, "/justmart.exe") {
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			tmp := dest + ".part"
			out, err := os.Create(tmp)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, rc); err != nil { //nolint:gosec // size bounded by our own release zip
				out.Close()
				os.Remove(tmp)
				return err
			}
			if err := out.Close(); err != nil {
				os.Remove(tmp)
				return err
			}
			return os.Rename(tmp, dest)
		}
	}
	return errors.New("justmart.exe not found in the update zip")
}

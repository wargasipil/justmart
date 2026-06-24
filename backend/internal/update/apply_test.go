package update

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// makePortableZip returns a zip whose only meaningful entry is
// justmart-portable-<ver>/justmart.exe with the given exe bytes.
func makePortableZip(t *testing.T, exeContent []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("justmart-portable-1.3.0/justmart.exe")
	require.NoError(t, err)
	_, err = w.Write(exeContent)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// updateServer serves the zip at /a.zip and a checksum at /a.zip.sha256 (the
// real sha256 unless overridden), mimicking the GitHub release assets.
func updateServer(t *testing.T, zipBytes []byte, checksumOverride string) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(zipBytes)
	checksum := hex.EncodeToString(sum[:])
	if checksumOverride != "" {
		checksum = checksumOverride
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a.zip":
			_, _ = w.Write(zipBytes)
		case "/a.zip.sha256":
			_, _ = w.Write([]byte(checksum + "  justmart-portable-1.3.0.zip\n")) // hex + filename
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestApply_StagesNewExe(t *testing.T) {
	t.Parallel()
	exeBytes := []byte("NEW-BINARY-v1.3.0")
	zipBytes := makePortableZip(t, exeBytes)
	srv := updateServer(t, zipBytes, "")
	defer srv.Close()

	exeDir := t.TempDir()
	info := Info{Latest: "1.3.0", AssetURL: srv.URL + "/a.zip", ChecksumURL: srv.URL + "/a.zip.sha256"}

	staged, err := Apply(context.Background(), srv.Client(), info, exeDir)
	require.NoError(t, err)
	require.Equal(t, "1.3.0", staged)

	got, err := os.ReadFile(filepath.Join(exeDir, StagedName))
	require.NoError(t, err)
	require.Equal(t, exeBytes, got)              // the new exe was extracted + staged
	_, err = os.Stat(filepath.Join(exeDir, ".update"))
	require.True(t, os.IsNotExist(err))          // work dir cleaned up
}

func TestApply_RejectsBadChecksum(t *testing.T) {
	t.Parallel()
	zipBytes := makePortableZip(t, []byte("whatever"))
	srv := updateServer(t, zipBytes, strings.Repeat("a", 64)) // 64 hex chars, wrong
	defer srv.Close()

	exeDir := t.TempDir()
	info := Info{Latest: "1.3.0", AssetURL: srv.URL + "/a.zip", ChecksumURL: srv.URL + "/a.zip.sha256"}

	_, err := Apply(context.Background(), srv.Client(), info, exeDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "checksum mismatch")
	_, statErr := os.Stat(filepath.Join(exeDir, StagedName))
	require.True(t, os.IsNotExist(statErr)) // nothing staged on a bad checksum
}

func TestApply_RefusesWithoutAssets(t *testing.T) {
	t.Parallel()
	_, err := Apply(context.Background(), http.DefaultClient, Info{Latest: "1.3.0"}, t.TempDir())
	require.Error(t, err)
}

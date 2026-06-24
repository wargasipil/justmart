package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeGitHub serves a canned releases/latest payload for the given tag + assets.
func fakeGitHub(t *testing.T, tag string, withAssets bool) *httptest.Server {
	t.Helper()
	assets := "[]"
	if withAssets {
		assets = `[
			{"name":"justmart-portable-` + tag + `.zip","browser_download_url":"https://dl/justmart-portable-` + tag + `.zip"},
			{"name":"justmart-portable-` + tag + `.zip.sha256","browser_download_url":"https://dl/justmart-portable-` + tag + `.zip.sha256"}
		]`
	}
	body := `{"tag_name":"v` + tag + `","body":"notes","html_url":"https://gh/rel/` + tag + `","published_at":"2026-01-02T03:04:05Z","assets":` + assets + `}`
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/repos/owner/name/releases/latest", r.URL.Path)
		require.NotEmpty(t, r.Header.Get("User-Agent")) // GitHub requires a UA
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestCheck_UpdateAvailable(t *testing.T) {
	t.Parallel()
	srv := fakeGitHub(t, "1.3.0", true)
	defer srv.Close()

	info, err := Check(context.Background(), srv.Client(), srv.URL, "owner/name", "1.2.0")
	require.NoError(t, err)
	require.True(t, info.Checked)
	require.True(t, info.UpdateAvailable)
	require.Equal(t, "1.2.0", info.Current)
	require.Equal(t, "1.3.0", info.Latest)
	require.Equal(t, "notes", info.ReleaseNotes)
	require.Equal(t, "https://gh/rel/1.3.0", info.ReleaseURL)
	require.NotZero(t, info.PublishedAt)
	require.Equal(t, "https://dl/justmart-portable-1.3.0.zip", info.AssetURL)
	require.Equal(t, "https://dl/justmart-portable-1.3.0.zip.sha256", info.ChecksumURL)
}

func TestCheck_UpToDate(t *testing.T) {
	t.Parallel()
	srv := fakeGitHub(t, "1.2.0", true)
	defer srv.Close()

	info, err := Check(context.Background(), srv.Client(), srv.URL, "owner/name", "1.2.0")
	require.NoError(t, err)
	require.True(t, info.Checked)
	require.False(t, info.UpdateAvailable)
	require.Equal(t, "1.2.0", info.Latest)
}

// Numeric per-segment compare: 1.10.0 must be newer than 1.9.0 (not lexical).
func TestCheck_NumericSegmentCompare(t *testing.T) {
	t.Parallel()
	srv := fakeGitHub(t, "1.10.0", true)
	defer srv.Close()

	info, err := Check(context.Background(), srv.Client(), srv.URL, "owner/name", "1.9.0")
	require.NoError(t, err)
	require.True(t, info.UpdateAvailable)
}

// A "dev" current never nags, but the latest is still reported.
func TestCheck_DevBuildNeverNags(t *testing.T) {
	t.Parallel()
	srv := fakeGitHub(t, "1.3.0", true)
	defer srv.Close()

	info, err := Check(context.Background(), srv.Client(), srv.URL, "owner/name", "dev")
	require.NoError(t, err)
	require.True(t, info.Checked)
	require.False(t, info.UpdateAvailable)
	require.Equal(t, "1.3.0", info.Latest)
}

// No portable/sha256 assets → URLs empty (self-apply will be unavailable).
func TestCheck_NoAssets(t *testing.T) {
	t.Parallel()
	srv := fakeGitHub(t, "1.3.0", false)
	defer srv.Close()

	info, err := Check(context.Background(), srv.Client(), srv.URL, "owner/name", "1.2.0")
	require.NoError(t, err)
	require.True(t, info.UpdateAvailable)
	require.Empty(t, info.AssetURL)
	require.Empty(t, info.ChecksumURL)
}

// HTTP 404 (repo has no releases) → soft fail: Checked=false + error, no panic.
func TestCheck_NotFoundSoftFails(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	info, err := Check(context.Background(), srv.Client(), srv.URL, "owner/name", "1.2.0")
	require.Error(t, err)
	require.False(t, info.Checked)
	require.Equal(t, "1.2.0", info.Current) // current still returned
}

func TestCompareVersions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b string
		sign int
		ok   bool
	}{
		{"1.3.0", "1.2.0", 1, true},
		{"1.2.0", "1.3.0", -1, true},
		{"1.2.0", "1.2.0", 0, true},
		{"1.10.0", "1.9.0", 1, true}, // numeric, not lexical
		{"2.0", "1.9.9", 1, true},
		{"1.2.0-rc1", "1.2.0", 0, true}, // suffix ignored
		{"v1.2.0", "1.2.0", 0, true},    // leading v stripped
		{"dev", "1.2.0", 0, false},      // unparseable
	}
	for _, c := range cases {
		sign, ok := compareVersions(c.a, c.b)
		require.Equal(t, c.ok, ok, "%s vs %s ok", c.a, c.b)
		if ok {
			require.Equal(t, c.sign, sign, "%s vs %s sign", c.a, c.b)
		}
	}
}

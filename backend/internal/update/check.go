// Package update implements the in-app autoupdater for the portable Windows
// build: a read-only check against GitHub Releases (check.go) and a one-click
// self-apply that downloads + verifies + stages the new binary (apply.go). It is
// DB-free and depends only on the standard library so it stays trivially
// unit-testable with an httptest server.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Info is the result of a release check. Checked is false when the remote lookup
// failed (offline / 404 / rate-limited) — callers then degrade gracefully rather
// than erroring. AssetURL/ChecksumURL drive self-apply (apply.go).
type Info struct {
	Current         string
	Latest          string
	UpdateAvailable bool
	ReleaseNotes    string
	ReleaseURL      string
	PublishedAt     int64
	Checked         bool
	AssetURL        string // the justmart-portable-*.zip download URL
	ChecksumURL     string // its *.sha256 download URL
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Check queries <apiBase>/repos/<repo>/releases/latest and compares the latest
// release tag to current. A network/HTTP/parse failure returns Info{Current,
// Checked:false} + the error (never a panic); the handler surfaces it softly.
func Check(ctx context.Context, client *http.Client, apiBase, repo, current string) (Info, error) {
	info := Info{Current: current}
	url := fmt.Sprintf("%s/repos/%s/releases/latest", strings.TrimRight(apiBase, "/"), repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("User-Agent", "justmart-updater")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// 404 = repo has no releases yet; 403 = rate-limited; etc. Soft-fail.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return info, fmt.Errorf("github releases/latest: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var rel ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return info, fmt.Errorf("decode release: %w", err)
	}

	info.Checked = true
	info.Latest = strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	info.ReleaseNotes = rel.Body
	info.ReleaseURL = rel.HTMLURL
	if !rel.PublishedAt.IsZero() {
		info.PublishedAt = rel.PublishedAt.Unix()
	}
	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		switch {
		case strings.HasSuffix(name, ".sha256"):
			info.ChecksumURL = a.BrowserDownloadURL
		case strings.HasPrefix(name, "justmart-portable") && strings.HasSuffix(name, ".zip"):
			info.AssetURL = a.BrowserDownloadURL
		}
	}

	// Only flag an update when both versions parse and latest > current. A "dev"
	// / unparseable current never nags (but Latest is still shown).
	if cmp, ok := compareVersions(info.Latest, current); ok && cmp > 0 {
		info.UpdateAvailable = true
	}
	return info, nil
}

// compareVersions returns (sign, ok): sign is +1 if a>b, -1 if a<b, 0 if equal;
// ok is false when either side isn't a numeric dotted version. Segments compare
// numerically (so 1.10.0 > 1.9.0), a pre-release/build suffix (after '-' or '+')
// is ignored, and missing trailing segments count as 0.
func compareVersions(a, b string) (int, bool) {
	pa, oka := parseVersion(a)
	pb, okb := parseVersion(b)
	if !oka || !okb {
		return 0, false
	}
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			if x > y {
				return 1, true
			}
			return -1, true
		}
	}
	return 0, true
}

func parseVersion(s string) ([]int, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	// Drop a pre-release / build-metadata suffix (1.2.0-rc1, 1.2.0+abc).
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return nil, false
	}
	parts := strings.Split(s, ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		out[i] = n
	}
	return out, true
}

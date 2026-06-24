package settings

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/config"
)

func fakeGitHubLatest(t *testing.T, tag string) *httptest.Server {
	t.Helper()
	body := `{"tag_name":"v` + tag + `","body":"notes","html_url":"https://gh/` + tag + `","published_at":"2026-01-02T03:04:05Z","assets":[]}`
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

// White-box so we can point updateAPIBase at the httptest server (no DB needed —
// CheckUpdate never touches it).
func TestCheckUpdate_Available(t *testing.T) {
	t.Parallel()
	srv := fakeGitHubLatest(t, "1.3.0")
	defer srv.Close()
	svc := &SettingsService{version: "1.2.0", updateCfg: config.Update{Repo: "owner/name"}, updateAPIBase: srv.URL}

	resp, err := svc.CheckUpdate(context.Background(), connect.NewRequest(&settingsifacev1.CheckUpdateRequest{}))
	require.NoError(t, err)
	require.Equal(t, "1.2.0", resp.Msg.CurrentVersion)
	require.True(t, resp.Msg.Enabled)
	require.True(t, resp.Msg.Checked)
	require.True(t, resp.Msg.UpdateAvailable)
	require.Equal(t, "1.3.0", resp.Msg.LatestVersion)
	require.Equal(t, "notes", resp.Msg.ReleaseNotes)
}

func TestCheckUpdate_Disabled(t *testing.T) {
	t.Parallel()
	svc := &SettingsService{version: "1.2.0", updateCfg: config.Update{Disabled: true, Repo: "owner/name"}, updateAPIBase: "http://unused"}

	resp, err := svc.CheckUpdate(context.Background(), connect.NewRequest(&settingsifacev1.CheckUpdateRequest{}))
	require.NoError(t, err)
	require.False(t, resp.Msg.Enabled)
	require.False(t, resp.Msg.Checked)
	require.Equal(t, "1.2.0", resp.Msg.CurrentVersion)
}

// A remote failure (here: a closed server) is soft — checked=false, no error.
func TestCheckUpdate_RemoteFailureSoft(t *testing.T) {
	t.Parallel()
	srv := fakeGitHubLatest(t, "1.3.0")
	srv.Close() // force a connection error
	svc := &SettingsService{version: "1.2.0", updateCfg: config.Update{Repo: "owner/name"}, updateAPIBase: srv.URL}

	resp, err := svc.CheckUpdate(context.Background(), connect.NewRequest(&settingsifacev1.CheckUpdateRequest{}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Enabled)
	require.False(t, resp.Msg.Checked)
	require.Equal(t, "1.2.0", resp.Msg.CurrentVersion)
}

// version "" reports "dev".
func TestCheckUpdate_DevDefault(t *testing.T) {
	t.Parallel()
	svc := &SettingsService{updateCfg: config.Update{Disabled: true}}
	resp, err := svc.CheckUpdate(context.Background(), connect.NewRequest(&settingsifacev1.CheckUpdateRequest{}))
	require.NoError(t, err)
	require.Equal(t, "dev", resp.Msg.CurrentVersion)
}

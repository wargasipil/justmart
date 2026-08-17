package settings_test

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
	settingssvc "github.com/justmart/backend/internal/service/settings"
)

// A real token's shape (base64url, ~180 chars). Not a credential.
const tunnelTok = "eyJhIjoiMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMCIsInQiOiIxMTExMTExMS0yMjIyLTMzMzMtNDQ0NC01NTU1NTU1NTU1NTUiLCJzIjoiWm05dlltRnlZbUY2Ykc5c1kyRjBaR1Y2ZW1FPSJ9"

const tunnelTok2 = "eyJhIjoiOTk5OTk5OTk5OTk5OTk5OTk5OTk5OTk5OTk5OSIsInQiOiJhYWFhYWFhYS1iYmJiLWNjY2MtZGRkZC1lZWVlZWVlZWVlZWUiLCJzIjoiVFdGeWEyVjBjR3hoWTJVdGMyVmpjbVYwTFd0bGVRPT0ifQ"

func newTunnelSvc(t *testing.T) *settingssvc.SettingsService {
	t.Helper()
	return settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
}

func getTunnel(t *testing.T, svc *settingssvc.SettingsService) *settingsifacev1.GetTunnelSettingsResponse {
	t.Helper()
	resp, err := svc.GetTunnelSettings(context.Background(),
		connect.NewRequest(&settingsifacev1.GetTunnelSettingsRequest{}))
	require.NoError(t, err)
	return resp.Msg
}

// A fresh shop has no tunnel and nothing saved.
func TestGetTunnelSettings_DefaultWhenUnset(t *testing.T) {
	t.Parallel()
	got := getTunnel(t, newTunnelSvc(t))
	require.False(t, got.Configured)
	require.Empty(t, got.TokenPreview)
	require.False(t, got.Active)
	require.Equal(t, "none", got.Source)
	require.False(t, got.RestartRequired)
}

// Saving a token persists it, reports the settings source, and asks for a
// restart — the tunnel is launched once at boot, so nothing is running yet.
func TestSetTunnelSettings_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := newTunnelSvc(t)

	set, err := svc.SetTunnelSettings(context.Background(),
		connect.NewRequest(&settingsifacev1.SetTunnelSettingsRequest{Token: tunnelTok}))
	require.NoError(t, err)
	require.True(t, set.Msg.Configured)
	require.Equal(t, "settings", set.Msg.Source)
	require.True(t, set.Msg.RestartRequired)
	require.False(t, set.Msg.Active)

	got := getTunnel(t, svc)
	require.True(t, got.Configured)
	require.Equal(t, "settings", got.Source)
	require.True(t, got.RestartRequired)
}

// The token must never come back over the wire — the panel that renders this
// response is a browser tab that may be screen-shared or recorded.
func TestGetTunnelSettings_NeverReturnsTheRawToken(t *testing.T) {
	t.Parallel()
	svc := newTunnelSvc(t)
	_, err := svc.SetTunnelSettings(context.Background(),
		connect.NewRequest(&settingsifacev1.SetTunnelSettingsRequest{Token: tunnelTok}))
	require.NoError(t, err)

	got := getTunnel(t, svc)
	require.NotEqual(t, tunnelTok, got.TokenPreview)
	require.NotContains(t, got.TokenPreview, tunnelTok[6:len(tunnelTok)-4])
	require.Contains(t, got.TokenPreview, "•")
}

// The field takes the token, not the `cloudflared service install <token>`
// command Cloudflare shows next to it — the most likely wrong paste, and the
// one the error message has to be understandable for.
func TestSetTunnelSettings_RejectsAPastedInstallCommand(t *testing.T) {
	t.Parallel()
	svc := newTunnelSvc(t)

	_, err := svc.SetTunnelSettings(context.Background(),
		connect.NewRequest(&settingsifacev1.SetTunnelSettingsRequest{
			Token: "cloudflared.exe service install " + tunnelTok,
		}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, "settings.tunnel_token_invalid", ce.Message())

	// And nothing was stored from the half that WAS a token.
	require.False(t, getTunnel(t, svc).Configured)
}

func TestSetTunnelSettings_RejectsGarbage(t *testing.T) {
	t.Parallel()
	svc := newTunnelSvc(t)

	for _, bad := range []string{"nope", "token saya", strings.Repeat("a", 40) + "!"} {
		_, err := svc.SetTunnelSettings(context.Background(),
			connect.NewRequest(&settingsifacev1.SetTunnelSettingsRequest{Token: bad}))
		require.Error(t, err, bad)
		require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), bad)

		var ce *connect.Error
		require.ErrorAs(t, err, &ce)
		require.Equal(t, "settings.tunnel_token_invalid", ce.Message(), bad)
	}

	// A rejected token must not have been written.
	require.False(t, getTunnel(t, svc).Configured)
}

// Clearing turns the tunnel off for the next boot without needing a config edit.
func TestSetTunnelSettings_EmptyClears(t *testing.T) {
	t.Parallel()
	svc := newTunnelSvc(t)
	ctx := context.Background()

	_, err := svc.SetTunnelSettings(ctx,
		connect.NewRequest(&settingsifacev1.SetTunnelSettingsRequest{Token: tunnelTok}))
	require.NoError(t, err)

	cleared, err := svc.SetTunnelSettings(ctx,
		connect.NewRequest(&settingsifacev1.SetTunnelSettingsRequest{Token: "  "}))
	require.NoError(t, err)
	require.False(t, cleared.Msg.Configured)
	require.Empty(t, cleared.Msg.TokenPreview)
	require.Equal(t, "none", cleared.Msg.Source)
}

// A server already running the saved token is settled: nothing to restart for.
func TestGetTunnelSettings_ActiveOnTheSavedToken(t *testing.T) {
	t.Parallel()
	svc := newTunnelSvc(t)
	_, err := svc.SetTunnelSettings(context.Background(),
		connect.NewRequest(&settingsifacev1.SetTunnelSettingsRequest{Token: tunnelTok}))
	require.NoError(t, err)

	svc.SetTunnel(tunnelTok, "", false, false) // as if this process booted on it

	got := getTunnel(t, svc)
	require.True(t, got.Active)
	require.False(t, got.RestartRequired)
	require.Equal(t, "settings", got.Source)
}

// Editing the token while a tunnel is up is exactly the case the restart notice
// exists for: the old tunnel keeps running until the app is restarted.
func TestGetTunnelSettings_ChangingTheTokenAsksForARestart(t *testing.T) {
	t.Parallel()
	svc := newTunnelSvc(t)
	ctx := context.Background()

	_, err := svc.SetTunnelSettings(ctx,
		connect.NewRequest(&settingsifacev1.SetTunnelSettingsRequest{Token: tunnelTok}))
	require.NoError(t, err)
	svc.SetTunnel(tunnelTok, "", false, false)

	changed, err := svc.SetTunnelSettings(ctx,
		connect.NewRequest(&settingsifacev1.SetTunnelSettingsRequest{Token: tunnelTok2}))
	require.NoError(t, err)
	require.True(t, changed.Msg.Active)          // the OLD tunnel is still up
	require.True(t, changed.Msg.RestartRequired) // ...on the OLD token
}

// config.yaml still works, and the panel says so rather than showing "none"
// next to a shop that is demonstrably reachable.
func TestGetTunnelSettings_ConfigFileTokenIsReported(t *testing.T) {
	t.Parallel()
	svc := newTunnelSvc(t)
	svc.SetTunnel(tunnelTok, tunnelTok, false, false)

	got := getTunnel(t, svc)
	require.False(t, got.Configured) // nothing saved in Settings
	require.True(t, got.Active)
	require.Equal(t, "config", got.Source)
	require.False(t, got.RestartRequired)
}

// The env off-switch is the one thing the UI cannot beat — recording an FAQ
// video and running the browser suite both rely on it, against the dev database.
func TestGetTunnelSettings_EnvOffBeatsASavedToken(t *testing.T) {
	t.Parallel()
	svc := newTunnelSvc(t)
	_, err := svc.SetTunnelSettings(context.Background(),
		connect.NewRequest(&settingsifacev1.SetTunnelSettingsRequest{Token: tunnelTok}))
	require.NoError(t, err)

	svc.SetTunnel("", "", false, true) // JUSTMART_CLOUDFLARE_TUNNEL_TOKEN=off

	got := getTunnel(t, svc)
	require.True(t, got.Configured) // saved...
	require.False(t, got.Active)    // ...but overruled
	require.Equal(t, "env_off", got.Source)
	// No restart would help, so don't ask for one.
	require.False(t, got.RestartRequired)
}

// An env token also outranks the saved one; the panel reports which is winning.
func TestGetTunnelSettings_EnvTokenOutranksTheSavedOne(t *testing.T) {
	t.Parallel()
	svc := newTunnelSvc(t)
	_, err := svc.SetTunnelSettings(context.Background(),
		connect.NewRequest(&settingsifacev1.SetTunnelSettingsRequest{Token: tunnelTok}))
	require.NoError(t, err)

	svc.SetTunnel(tunnelTok2, tunnelTok2, true, false)

	got := getTunnel(t, svc)
	require.True(t, got.Configured)
	require.True(t, got.Active)
	require.Equal(t, "env", got.Source)
	require.False(t, got.RestartRequired)
}

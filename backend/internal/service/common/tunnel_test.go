package common_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/service/servicetest"
)

// A real token, shape-wise: base64url, ~180 chars. Not a credential.
const fakeToken = "eyJhIjoiMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMCIsInQiOiIxMTExMTExMS0yMjIyLTMzMzMtNDQ0NC01NTU1NTU1NTU1NTUiLCJzIjoiWm05dlltRnlZbUY2Ykc5c1kyRjBaR1Y2ZW1FPSJ9"

func TestNormalizeTunnelToken(t *testing.T) {
	t.Parallel()

	t.Run("bare token passes through", func(t *testing.T) {
		got, ok := common.NormalizeTunnelToken(fakeToken)
		require.True(t, ok)
		require.Equal(t, fakeToken, got)
	})

	t.Run("surrounding whitespace is trimmed", func(t *testing.T) {
		got, ok := common.NormalizeTunnelToken("  \n" + fakeToken + "\t ")
		require.True(t, ok)
		require.Equal(t, fakeToken, got)
	})

	// The field takes the token, not the install command Cloudflare shows next
	// to it. Digging the token out of a longer string would mean storing a
	// credential nobody checked, so the whole input is refused instead.
	t.Run("rejects a pasted install command", func(t *testing.T) {
		for _, pasted := range []string{
			"cloudflared.exe service install " + fakeToken,
			"cloudflared service install " + fakeToken,
			`cloudflared.exe service install "` + fakeToken + `"`,
			"sudo cloudflared service install " + fakeToken,
			`"` + fakeToken + `"`, // quoted, as PowerShell examples show it
		} {
			_, ok := common.NormalizeTunnelToken(pasted)
			require.False(t, ok, pasted)
		}
	})

	t.Run("empty clears the setting", func(t *testing.T) {
		got, ok := common.NormalizeTunnelToken("   ")
		require.True(t, ok)
		require.Equal(t, "", got)
	})

	t.Run("rejects what is not a token", func(t *testing.T) {
		for _, bad := range []string{
			"nope",                            // too short
			"tunnel saya",                     // prose, and the last field is short
			strings.Repeat("a", 31),           // just under the floor
			strings.Repeat("a", 4097),         // over the ceiling
			strings.Repeat("a", 40) + "!",     // not base64url
			strings.Repeat("a", 40) + "é", // non-ASCII
		} {
			_, ok := common.NormalizeTunnelToken(bad)
			require.False(t, ok, "%q should be rejected", bad)
		}
	})
}

// The masked form must be recognizable and unusable — it is rendered in a
// browser tab that may end up in a screen share or a screen recording.
func TestMaskTunnelToken(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", common.MaskTunnelToken(""))

	masked := common.MaskTunnelToken(fakeToken)
	require.NotContains(t, masked, fakeToken[6:len(fakeToken)-4])
	require.True(t, strings.HasPrefix(masked, fakeToken[:6]))
	require.True(t, strings.HasSuffix(masked, fakeToken[len(fakeToken)-4:]))
	require.Less(t, len(masked), len(fakeToken))

	// A short value must not leak proportionally more of itself.
	require.Equal(t, "••••••", common.MaskTunnelToken("abcdef"))
}

func TestResolveTunnel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("nothing configured means no tunnel", func(t *testing.T) {
		t.Parallel()
		db, _ := servicetest.New(t)
		st, err := common.ResolveTunnel(ctx, db, "", false, false)
		require.NoError(t, err)
		require.Equal(t, "", st.Token)
		require.Equal(t, common.TunnelSourceNone, st.Source)
	})

	t.Run("config.yaml is used when nothing is saved", func(t *testing.T) {
		t.Parallel()
		db, _ := servicetest.New(t)
		st, err := common.ResolveTunnel(ctx, db, "from-config", false, false)
		require.NoError(t, err)
		require.Equal(t, "from-config", st.Token)
		require.Equal(t, common.TunnelSourceConfig, st.Source)
	})

	t.Run("a saved token outranks config.yaml", func(t *testing.T) {
		t.Parallel()
		db, _ := servicetest.New(t)
		require.NoError(t, common.SetCloudflareTunnelToken(ctx, db, "from-settings"))

		st, err := common.ResolveTunnel(ctx, db, "from-config", false, false)
		require.NoError(t, err)
		require.Equal(t, "from-settings", st.Token)
		require.Equal(t, common.TunnelSourceSettings, st.Source)
		require.Equal(t, "from-settings", st.Saved)
	})

	t.Run("the environment outranks a saved token", func(t *testing.T) {
		t.Parallel()
		db, _ := servicetest.New(t)
		require.NoError(t, common.SetCloudflareTunnelToken(ctx, db, "from-settings"))

		st, err := common.ResolveTunnel(ctx, db, "from-env", true, false)
		require.NoError(t, err)
		require.Equal(t, "from-env", st.Token)
		require.Equal(t, common.TunnelSourceEnv, st.Source)
		// Still reported, so the panel can say "saved, but overridden".
		require.Equal(t, "from-settings", st.Saved)
	})

	// The load-bearing one. Recording an FAQ video and running the browser suite
	// both boot a real server against the dev database with the env off-switch
	// set; if a token typed into the UI could beat it, those runs would publish
	// the dev database on a public hostname.
	t.Run("the environment off-switch beats everything", func(t *testing.T) {
		t.Parallel()
		db, _ := servicetest.New(t)
		require.NoError(t, common.SetCloudflareTunnelToken(ctx, db, fakeToken))

		st, err := common.ResolveTunnel(ctx, db, "", false, true)
		require.NoError(t, err)
		require.Equal(t, "", st.Token)
		require.Equal(t, common.TunnelSourceEnvOff, st.Source)
	})

	t.Run("clearing the saved token falls back to config.yaml", func(t *testing.T) {
		t.Parallel()
		db, _ := servicetest.New(t)
		require.NoError(t, common.SetCloudflareTunnelToken(ctx, db, "from-settings"))
		require.NoError(t, common.SetCloudflareTunnelToken(ctx, db, ""))

		st, err := common.ResolveTunnel(ctx, db, "from-config", false, false)
		require.NoError(t, err)
		require.Equal(t, "from-config", st.Token)
		require.Equal(t, common.TunnelSourceConfig, st.Source)
	})
}

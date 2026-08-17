package common

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

// SettingKeyCloudflareTunnelToken stores the Cloudflare Tunnel token an OWNER
// saved in Settings ▸ Remote access. Empty (or absent) = nothing saved here;
// the tunnel then falls back to config.yaml / the environment.
//
// It is a credential living in the database, which is a deliberate trade: the
// alternative is telling a shop owner to edit YAML next to the exe, and the
// only people who can read app_settings already have the whole shop's data.
// It is never handed back to the browser — see MaskTunnelToken.
const SettingKeyCloudflareTunnelToken = "cloudflare_tunnel_token"

// TunnelSource names where the token the server actually runs on came from.
// The Settings panel shows it, because "I saved a token and nothing happened"
// has three different causes and they need different fixes.
type TunnelSource string

const (
	// TunnelSourceNone — nothing configured anywhere; no tunnel.
	TunnelSourceNone TunnelSource = "none"
	// TunnelSourceSettings — the token saved in Settings ▸ Remote access.
	TunnelSourceSettings TunnelSource = "settings"
	// TunnelSourceConfig — config.yaml cloudflare_tunnel_token.
	TunnelSourceConfig TunnelSource = "config"
	// TunnelSourceEnv — $JUSTMART_CLOUDFLARE_TUNNEL_TOKEN, which outranks both.
	TunnelSourceEnv TunnelSource = "env"
	// TunnelSourceEnvOff — the environment explicitly disables the tunnel
	// ("off"/"none"/"false"/"0"/"disabled"). Nothing can turn it back on
	// without restarting the process without that variable.
	TunnelSourceEnvOff TunnelSource = "env_off"
)

// TunnelState is the resolved answer to "does this server run a tunnel, on
// which token, and decided by whom".
type TunnelState struct {
	// Token is the effective token. Empty means no tunnel.
	Token string
	// Source is where Token came from.
	Source TunnelSource
	// Saved is the app_settings token regardless of whether it won — the panel
	// still shows a saved-but-shadowed token, otherwise saving it looks like it
	// silently failed.
	Saved string
}

// ResolveTunnel decides which Cloudflare token this process should run on.
//
// Order, highest first:
//
//	1. environment says off        -> no tunnel, and nothing overrides it
//	2. $JUSTMART_CLOUDFLARE_TUNNEL_TOKEN
//	3. Settings ▸ Remote access (app_settings)
//	4. config.yaml cloudflare_tunnel_token
//
// The environment stays on top in BOTH directions on purpose. Recording the FAQ
// videos and running the browser suite both drive a real server against the dev
// database with `JUSTMART_CLOUDFLARE_TUNNEL_TOKEN=off`; if a token typed into
// the UI could outrank that, a recording session would publish the dev database
// on a public hostname. The off-switch has to be the one thing the UI cannot
// beat. (cfgToken already carries the env value when envSet — config.Load
// collapses them — which is why the flags are passed alongside it.)
//
// A read failure is not fatal: the caller gets the config/env answer plus the
// error, so a server whose app_settings table is unreadable still boots the way
// it did before this setting existed.
func ResolveTunnel(ctx context.Context, db *gorm.DB, cfgToken string, envSet, envOff bool) (TunnelState, error) {
	saved, err := GetCloudflareTunnelToken(ctx, db)
	if err == nil && envOff {
		// Saved is still reported even though it lost: an owner who saved a
		// token and then sees "nothing configured" concludes the save failed and
		// saves it again. The panel needs to say "saved, but the environment
		// disables the tunnel on this machine", which needs both halves.
		return TunnelState{Source: TunnelSourceEnvOff, Saved: saved}, nil
	}
	if err != nil && envOff {
		return TunnelState{Source: TunnelSourceEnvOff}, err
	}
	if err != nil {
		// Fall back to the pre-settings behavior rather than refusing to boot.
		state := TunnelState{Token: cfgToken}
		switch {
		case envSet:
			state.Source = TunnelSourceEnv
		case cfgToken != "":
			state.Source = TunnelSourceConfig
		default:
			state.Source = TunnelSourceNone
		}
		return state, err
	}

	switch {
	case envSet:
		return TunnelState{Token: cfgToken, Source: TunnelSourceEnv, Saved: saved}, nil
	case saved != "":
		return TunnelState{Token: saved, Source: TunnelSourceSettings, Saved: saved}, nil
	case cfgToken != "":
		return TunnelState{Token: cfgToken, Source: TunnelSourceConfig, Saved: saved}, nil
	default:
		return TunnelState{Source: TunnelSourceNone, Saved: saved}, nil
	}
}

// GetCloudflareTunnelToken returns the token saved in Settings ("" when unset).
func GetCloudflareTunnelToken(ctx context.Context, db *gorm.DB) (string, error) {
	return getSetting(ctx, db, SettingKeyCloudflareTunnelToken)
}

// SetCloudflareTunnelToken persists the token. The value is stored verbatim —
// normalize it with NormalizeTunnelToken first.
func SetCloudflareTunnelToken(ctx context.Context, db *gorm.DB, token string) error {
	return setSetting(ctx, db, SettingKeyCloudflareTunnelToken, token)
}

// NormalizeTunnelToken trims surrounding whitespace and reports whether what is
// left is plausibly a token.
//
// The field takes the TOKEN ONLY — not the `cloudflared service install <token>`
// command line Cloudflare shows next to it. Whitespace, quotes and command words
// are all rejected by the charset check below rather than being stripped out:
// silently pulling a token out of a longer string means accepting input nobody
// checked, and a wrong guess would be stored as a credential and only surface
// later as a tunnel that will not come up.
//
// What remains is a shape check: base64url-ish and long. Whether Cloudflare
// accepts it is Cloudflare's answer to give, and the server reports that in the
// log when cloudflared starts.
func NormalizeTunnelToken(raw string) (token string, ok bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", true // clearing the setting is always valid
	}
	if len(s) < MinTunnelTokenLen || len(s) > MaxTunnelTokenLen {
		return "", false
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '+', r == '/', r == '=', r == '-', r == '_', r == '.':
		default:
			return "", false
		}
	}
	return s, true
}

const (
	// MinTunnelTokenLen is well under a real token (~180 chars of base64) but
	// high enough that a stray word can't be mistaken for a credential.
	MinTunnelTokenLen = 32
	// MaxTunnelTokenLen bounds what gets written into app_settings.
	MaxTunnelTokenLen = 4096
)

// MaskTunnelToken renders a token for display: enough to recognize which one is
// saved, never enough to use. The raw value never leaves the server once saved
// — a browser tab, a screen share, or an FAQ screen recording would otherwise
// carry a working key to the shop's network.
func MaskTunnelToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 12 {
		return strings.Repeat("•", len(token))
	}
	return token[:6] + "••••" + token[len(token)-4:]
}

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfig drops a minimal config.yaml in a temp dir and returns its path.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// cloudflare_tunnel_token is read from the YAML; absent means no tunnel, which
// is what App.Run keys off (a nil TunnelRunner).
func TestCloudflareTunnelToken_YAML(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"absent means no tunnel", "server:\n  port: 8080\n", ""},
		{"explicit empty means no tunnel", "cloudflare_tunnel_token: \"\"\n", ""},
		{"set", "cloudflare_tunnel_token: eyJhIjoidG9rZW4ifQ\n", "eyJhIjoidG9rZW4ifQ"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, c.body))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.CloudflareTunnelToken != c.want {
				t.Fatalf("CloudflareTunnelToken = %q, want %q", cfg.CloudflareTunnelToken, c.want)
			}
		})
	}
}

// The token is a credential, so the env override wins over the YAML value —
// same posture as JWT_SECRET / DB_PASSWORD. An empty env var is "not set" and
// leaves the YAML alone. Not parallel: mutates process env.
func TestCloudflareTunnelToken_EnvOverride(t *testing.T) {
	path := writeConfig(t, "cloudflare_tunnel_token: from-yaml\n")

	t.Setenv("JUSTMART_CLOUDFLARE_TUNNEL_TOKEN", "from-env")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CloudflareTunnelToken != "from-env" {
		t.Fatalf("env override: got %q, want %q", cfg.CloudflareTunnelToken, "from-env")
	}

	t.Setenv("JUSTMART_CLOUDFLARE_TUNNEL_TOKEN", "")
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CloudflareTunnelToken != "from-yaml" {
		t.Fatalf("empty env: got %q, want %q", cfg.CloudflareTunnelToken, "from-yaml")
	}
}

// The env var must also be able to say NO. Empty already means "not set" (see
// above), so without a sentinel there was no way to override a token that is
// already in config.yaml — and an automated local run (recording an FAQ video,
// a test pass) would open a public tunnel to the dev database as a side effect.
// Not parallel: mutates process env.
func TestCloudflareTunnelToken_EnvCanDisable(t *testing.T) {
	path := writeConfig(t, "cloudflare_tunnel_token: from-yaml\n")

	for _, v := range []string{"off", "OFF", "none", "false", "0", "disabled", " off "} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("JUSTMART_CLOUDFLARE_TUNNEL_TOKEN", v)
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.CloudflareTunnelToken != "" {
				t.Fatalf("%q should disable the tunnel, got %q", v, cfg.CloudflareTunnelToken)
			}
		})
	}

	// A real token is a long base64 blob; nothing about it should look like a
	// disable sentinel.
	t.Setenv("JUSTMART_CLOUDFLARE_TUNNEL_TOKEN", "eyJhIjoidG9rZW4ifQ")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CloudflareTunnelToken != "eyJhIjoidG9rZW4ifQ" {
		t.Fatalf("real token was swallowed: got %q", cfg.CloudflareTunnelToken)
	}
}

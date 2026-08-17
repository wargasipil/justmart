//go:build license

// This file is compiled ONLY into the licensed flavor (`go build -tags license`).
// It is the single place the getresolved SDK is imported, so an untagged build
// links none of it.

package licensegate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	license "github.com/wargasipil/getresolved_license"
)

// Enabled reports that this binary enforces a licence. Callers use it to skip
// starting the watcher entirely in an unlicensed build.
const Enabled = true

// ProductKey is the getresolved catalog key this binary IS.
//
// A CONSTANT, deliberately, and not a config field: every product in the
// catalog verifies against the same signing key, so a licence bought for
// another product carries a perfectly valid signature here — the only thing
// separating them is this string being compared to the claim's app_key. A build
// that read its own identity from config.yaml would let an operator rename it
// into whatever their licence happens to say.
const ProductKey = "justmart"

// ErrUnlicensed is returned by Verify when the install is definitively not
// licensed (as opposed to a misconfiguration or an aborted wait).
var ErrUnlicensed = errors.New("this install is not licensed")

// installTokenPath is the durable per-machine credential minted at activation.
// Its presence is what makes a licensed install boot straight through instead
// of showing the activation page again.
func installTokenPath(o Options) string { return filepath.Join(o.CacheDir, "install.token") }

// claimCachePath is the last-good signed claim. Persisting it is what lets a
// cold start during an outage still answer from the grace window.
func claimCachePath(o Options) string { return filepath.Join(o.CacheDir, "licence.token") }

// Verify is the fail-closed boot gate: it returns nil only when this install is
// licensed, and blocks in between serving a loopback activation page.
//
// It runs BEFORE the server binds its port, so an unlicensed install never
// serves the app — the only thing listening is the SDK's activation page on
// 127.0.0.1.
func Verify(ctx context.Context, o Options) error {
	o = o.withDefaults()

	// A licence id in config.yaml means "activate without asking". Best-effort:
	// on failure we fall through to the page, which shows the operator the
	// server's own message ("activation limit reached", "this licence has
	// expired") instead of a log line they will never read.
	if strings.TrimSpace(o.LicenceID) != "" {
		seedActivation(ctx, o)
	}

	st, err := license.InteractiveCheck(ctx, license.InteractiveOptions{
		ProductKey:       ProductKey,
		BaseURL:          o.BaseURL,
		TokenCache:       license.FileCache{Path: claimCachePath(o)},
		InstallTokenPath: installTokenPath(o),
		Grace:            o.Grace,
		Headless:         o.Headless,
		Timeout:          o.ActivationTimeout,
		OnServe: func(url string) {
			// Logged at Error, not Info: this is a stopped boot waiting on a
			// human, and on the portable flavor this line is what is visible in
			// the console window the launcher leaves open.
			slog.Error("justmart is not activated — open this page to enter your licence id", "url", url)
		},
	})
	if err != nil {
		return fmt.Errorf("licence activation: %w", err)
	}
	if !st.Valid {
		return fmt.Errorf("%w: %s", ErrUnlicensed, st.Reason)
	}

	attrs := []any{"source", st.Source, "reason", st.Reason}
	if st.Claim != nil {
		attrs = append(attrs, "licence_id", st.Claim.LicenceID, "tier", st.Claim.Tier, "billing", st.Claim.Billing)
		if st.Claim.ValidUntil != nil {
			attrs = append(attrs, "valid_until", st.Claim.ValidUntil.Format("2006-01-02"))
		}
	}
	slog.Info("licence verified", attrs...)
	return nil
}

// Watch re-verifies on a ticker and calls onInvalid once the licence stops
// being valid, so the caller can stop the server. It returns when ctx is done
// or after onInvalid has fired.
//
// A missing install token here is NOT treated as unlicensed: Verify has already
// run and passed, so this can only mean the token file went missing underneath
// a running server. Killing the till over that would be worse than the outage
// it is meant to prevent — the next boot's gate catches it.
func Watch(ctx context.Context, o Options, onInvalid func(reason string)) {
	o = o.withDefaults()
	client, err := newClient(o)
	if err != nil {
		slog.Warn("licence re-check disabled for this run", "err", err)
		return
	}
	watchLoop(ctx, o.RecheckInterval, func(ctx context.Context) (bool, string) {
		st := client.Check(ctx)
		if !st.Valid {
			return false, st.Reason
		}
		slog.Debug("licence still valid", "source", st.Source)
		return true, st.Reason
	}, onInvalid)
}

// newClient builds the verifying client from the install token Verify persisted.
func newClient(o Options) (*license.Client, error) {
	token, err := (license.FileCache{Path: installTokenPath(o)}).Load()
	if err != nil {
		return nil, fmt.Errorf("read install token: %w", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("install token is empty")
	}
	// One fingerprint for both the assertion and the offline comparison, derived
	// once: a machine that changes underneath us (a NIC removed) must not
	// refresh against one device and verify against another.
	device, _ := license.DeviceID()
	return license.New(license.Config{
		ProductKey: ProductKey,
		Refresher: &license.HTTPRefresher{
			BaseURL:  o.BaseURL,
			DeviceID: device,
			Authorize: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+token)
			},
		},
		Cache:    license.FileCache{Path: claimCachePath(o)},
		Grace:    o.Grace,
		DeviceID: device,
	})
}

// seedActivation exchanges a configured licence id for an install token, unless
// this machine already holds one. Re-activating the same machine is free on the
// server, but skipping it keeps an offline boot from stalling on a network call
// it does not need.
func seedActivation(ctx context.Context, o Options) {
	store := license.FileCache{Path: installTokenPath(o)}
	if tok, err := store.Load(); err == nil && strings.TrimSpace(tok) != "" {
		return
	}
	tok, err := license.Activate(ctx, o.BaseURL, strings.TrimSpace(o.LicenceID), ProductKey, nil)
	if err != nil {
		slog.Warn("could not activate the configured licence id", "err", err)
		return
	}
	if err := store.Save(tok); err != nil {
		slog.Warn("could not persist the install token", "err", err)
	}
}

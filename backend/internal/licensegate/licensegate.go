// Package licensegate is the getresolved licence guard for the LICENSED
// distribution flavor (portable + SQLite + licence).
//
// It exists as a build-tag pair so that the licensing SDK is linked ONLY into
// the licensed flavor, exactly like cmd/connector isolates the Windows spooler:
//
//	gate_enabled.go   //go:build license   — the real guard (imports the SDK)
//	gate_disabled.go  //go:build !license  — no-ops, zero dependencies
//
// So `make build` / `make dist-windows` / the Docker image are byte-for-byte
// unaffected — `go list -deps ./cmd/server` on an untagged build contains no
// getresolved_license — while `go build -tags license` produces a binary that
// refuses to serve without a valid licence. Callers (cmd/server) call Verify and
// Watch unconditionally; the tag decides whether anything happens.
//
// This file holds only what BOTH builds share: the options struct and the watch
// loop. Nothing here imports the SDK, which is what keeps the loop's semantics
// testable without a licensing server.
package licensegate

import (
	"context"
	"time"
)

// Default* are the fallbacks applied to a zero/partial Options. They live here
// rather than in the config package so the guard has sane behavior even if a
// hand-edited config.yaml omits the whole `license:` block.
const (
	DefaultBaseURL         = "https://api.getresolved.id"
	DefaultCacheDir        = "./license"
	DefaultGrace           = 72 * time.Hour
	DefaultRecheckInterval = 6 * time.Hour
)

// Options is everything the guard needs from configuration.
//
// Note what is NOT here: the product key. The SDK is emphatic that a build must
// not read its own identity from configuration — an operator who can set it can
// rename the product into whatever their licence happens to say — so it is a
// source constant (ProductKey in gate_enabled.go).
type Options struct {
	// BaseURL is the licensing server root.
	BaseURL string
	// LicenceID optionally pre-seeds activation (GR-XXXX-XXXX-XXXX-XXXX). When
	// set and this install is not yet activated, the guard activates it directly
	// and the operator never sees the activation page — for a scripted or
	// unattended install. Empty = ask in the browser.
	LicenceID string
	// CacheDir holds the durable install token and the last-good signed claim.
	// Losing it costs a re-activation on the same machine, not the licence.
	CacheDir string
	// Grace is how long a licensed install keeps running while the licensing
	// server is UNREACHABLE. It never extends a licence past its paid-through
	// date — that is refused immediately, online or off.
	Grace time.Duration
	// RecheckInterval is how often a running server re-verifies.
	RecheckInterval time.Duration
	// ActivationTimeout bounds how long boot waits for the operator to enter a
	// licence id. 0 waits until the process is signalled — the right default for
	// a shop PC that may be activated some minutes after it is switched on.
	ActivationTimeout time.Duration
	// Headless suppresses opening a browser at activation; the URL is logged
	// instead. For a box with no display.
	Headless bool
}

// withDefaults returns o with empty fields filled in.
func (o Options) withDefaults() Options {
	if o.BaseURL == "" {
		o.BaseURL = DefaultBaseURL
	}
	if o.CacheDir == "" {
		o.CacheDir = DefaultCacheDir
	}
	if o.Grace == 0 {
		o.Grace = DefaultGrace
	}
	if o.RecheckInterval <= 0 {
		o.RecheckInterval = DefaultRecheckInterval
	}
	return o
}

// watchLoop re-runs check on a ticker until ctx is done, and calls onInvalid
// exactly once with the reason the first time a check comes back invalid.
//
// It fires on the FIRST invalid verdict rather than after N strikes on purpose:
// the SDK's grace window is already the tolerance for a flaky network (an
// unreachable server keeps answering Valid until grace is exhausted), so a
// second layer of retries here would only extend an expired licence.
//
// Split out from the SDK call site so the stop semantics can be tested without
// a licensing server — the part that decides whether a shop's till dies mid-day
// should not be the part that is only exercised in production.
func watchLoop(
	ctx context.Context,
	interval time.Duration,
	check func(context.Context) (valid bool, reason string),
	onInvalid func(reason string),
) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if valid, reason := check(ctx); !valid {
				if onInvalid != nil {
					onInvalid(reason)
				}
				return
			}
		}
	}
}

package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/justmart/backend/internal/config"
)

// provideTunnelRunner returns nil unless a token was resolved — that nil is
// what makes App.Run skip the tunnel entirely.
func TestProvideTunnelRunner(t *testing.T) {
	t.Parallel()
	if got := provideTunnelRunner(""); got != nil {
		t.Fatal("no token configured: want a nil TunnelRunner, got non-nil")
	}
	if got := provideTunnelRunner("tok"); got == nil {
		t.Fatal("token configured: want a TunnelRunner, got nil")
	}
}

// A configured tunnel starts alongside the listener and is cancelled when the
// server shuts down — cloudflared must not outlive the process that spawned it.
func TestAppRun_StartsAndStopsTunnel(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})

	app := &App{
		Cfg:    &config.Config{},
		Server: &http.Server{Addr: "127.0.0.1:0"},
		Tunnel: func(ctx context.Context) error {
			close(started)
			<-ctx.Done() // the real runner blocks here until cloudflared exits
			close(stopped)
			return nil
		},
	}

	// Run installs its own signal handler on top of this ctx; cancelling the
	// parent is the in-process stand-in for SIGTERM.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("tunnel was never started")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Run did not return after shutdown")
	}

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("tunnel was not cancelled on shutdown")
	}
}

// With no token the graph hands App a nil TunnelRunner; Run must serve and shut
// down normally rather than panic on it.
func TestAppRun_NoTunnel(t *testing.T) {
	app := &App{
		Cfg:    &config.Config{},
		Server: &http.Server{Addr: "127.0.0.1:0"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()

	time.Sleep(50 * time.Millisecond) // let ListenAndServe get going
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Run did not return after shutdown")
	}
}

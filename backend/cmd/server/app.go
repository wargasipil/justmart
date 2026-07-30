package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/gorm"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/service/sale"
	"github.com/justmart/backend/internal/service/user"
)

// TunnelRunner starts the Cloudflare tunnel and blocks until ctx is cancelled
// or cloudflared exits. nil when no cloudflare_tunnel_token is configured — the
// seam also lets a test substitute a fake without spawning a subprocess.
type TunnelRunner func(ctx context.Context) error

// App is the fully-wired server: every dependency constructed (by Wire — see
// wire.go / wire_gen.go), nothing started yet. Boot performs the one-off boot
// steps; Run listens and drains on signal.
type App struct {
	Cfg    *config.Config
	DB     *gorm.DB
	Users  *user.UserService
	Server *http.Server
	Tunnel TunnelRunner // nil = no cloudflare_tunnel_token configured
}

// Boot runs the side effects a fresh process needs before it serves traffic:
// the bootstrap owner, the set-if-absent receipt seeds, and the abandoned-DRAFT
// sweeper. Construction stays in the Wire graph; this is lifecycle.
func (a *App) Boot(ctx context.Context) error {
	if err := a.Users.EnsureBootstrapOwner(ctx, a.Cfg.Bootstrap); err != nil {
		return fmt.Errorf("bootstrap owner: %w", err) // server can't start without it
	}

	// Seed the receipt header/footer from config.yaml into app_settings on first
	// boot (set-if-absent), so the printed receipt is editable in Settings ▸
	// Printing while existing config-based shops keep their values.
	if err := common.SeedReceiptDefaults(ctx, a.DB, a.Cfg.Printer.Header, a.Cfg.Printer.Footer); err != nil {
		slog.Warn("could not seed receipt defaults", "error", err)
	}
	// Seed the receipt paper width from config.yaml printer.width (set-if-absent),
	// so a config-based shop keeps its width and it's editable in Settings.
	if err := common.SeedReceiptWidth(ctx, a.DB, a.Cfg.Printer.Width); err != nil {
		slog.Warn("could not seed receipt width", "error", err)
	}

	// Background sweeper: hard-delete abandoned DRAFT carts the POS client missed
	// (crashes, lost sessions). In-process, single-node — like the rate limiter.
	sale.StartDraftSweeper(a.DB)
	return nil
}

// Run serves until the process is signalled, then drains in-flight requests.
//
// Fly (and Docker/WinSW) send SIGTERM on every deploy, restart and machine
// migration. Without this the process dies mid-request: in-flight sales are
// cut, the detached audit-log goroutines are dropped, and SQLite is left to
// recover its WAL on next boot. Drain instead.
func (a *App) Run(ctx context.Context) error {
	shutdownCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Optional Cloudflare tunnel (cloudflare_tunnel_token set): run it for as
	// long as the server does. It is a supporting process, not the service — a
	// tunnel that fails to start is logged, never fatal, so the shop keeps
	// working on the LAN. Cancelling shutdownCtx kills cloudflared; tunnelDone
	// lets the shutdown path wait for it to be reaped.
	tunnelDone := a.startTunnel(shutdownCtx)

	errCh := make(chan error, 1)
	go func() {
		slog.Info("justmart listening", "addr", a.Server.Addr)
		if err := a.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-shutdownCtx.Done():
		stop() // restore default handling: a second signal kills immediately
		slog.Info("shutting down; draining in-flight requests")
		drainCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := a.Server.Shutdown(drainCtx); err != nil {
			slog.Warn("graceful shutdown timed out", "err", err)
			waitTunnel(tunnelDone)
			return a.Server.Close()
		}
		waitTunnel(tunnelDone)
		slog.Info("shutdown complete")
		return nil
	}
}

// startTunnel launches the Cloudflare tunnel in the background when one is
// configured, and returns a channel closed once cloudflared has exited (nil
// when there is no tunnel — waitTunnel then returns immediately).
func (a *App) startTunnel(ctx context.Context) <-chan struct{} {
	if a.Tunnel == nil {
		return nil
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		slog.Info("starting cloudflare tunnel")
		if err := a.Tunnel(ctx); err != nil {
			slog.Error("cloudflare tunnel stopped", "err", err)
			return
		}
		slog.Info("cloudflare tunnel stopped")
	}()
	return done
}

// waitTunnel blocks until cloudflared has exited, bounded so a wedged child
// process can't hold the shutdown open.
func waitTunnel(done <-chan struct{}) {
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		slog.Warn("cloudflare tunnel did not exit in time")
	}
}

// Command server is the Justmart binary. Run with no subcommand to start the
// HTTP server (Connect API + embedded SPA); use the `migrate` subcommand to run
// database migrations. Built on urfave/cli/v3 (same as cmd/license).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	// Embed the IANA timezone DB so `TZ=Asia/Jakarta` resolves even on a minimal
	// base image (distroless has no /usr/share/zoneinfo). The "today" boundary in
	// GetTodaySnapshot relies on time.Local being the shop's zone.
	_ "time/tzdata"

	"github.com/urfave/cli/v3"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cmd := &cli.Command{
		Name:  "justmart",
		Usage: "Justmart server — runs the HTTP server by default; `migrate` for DB migrations",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "path to config.yaml (else $JUSTMART_CONFIG / ./config.yaml)",
			},
		},
		Action:   serve, // default action: run the server
		Commands: []*cli.Command{migrateCmd},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

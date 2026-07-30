package main

import (
	"context"

	"github.com/urfave/cli/v3"
)

// serve is the default CLI action: boot and run the HTTP server (Connect API +
// embedded SPA). Construction is Wire's job (initApp, generated in wire_gen.go
// from the providers in providers.go); this only drives the lifecycle —
// one-off boot steps, then listen + drain.
func serve(ctx context.Context, cmd *cli.Command) error {
	app, cleanup, err := initApp(configPath(cmd.String("config")), buildVersion(version))
	if err != nil {
		return err
	}
	defer cleanup()

	if err := app.Boot(ctx); err != nil {
		return err
	}
	return app.Run(ctx)
}

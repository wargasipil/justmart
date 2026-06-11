package main

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/glebarez/go-sqlite" // registers the pure-Go "sqlite" database/sql driver
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/urfave/cli/v3"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/migrations"
)

// migrateCmd is the `migrate` subcommand: a goose pass-through over the embedded
// migrations. Ports the former standalone cmd/migrate. Uses a lazy database/sql
// connection (NOT db.Open, which pings) so `create` works without a live DB.
var migrateCmd = &cli.Command{
	Name:      "migrate",
	Usage:     "Run database migrations (goose)",
	ArgsUsage: "[up|down|status|version|reset|create <name> sql]   (default: up)",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "path to config.yaml (else $JUSTMART_CONFIG / ./config.yaml)",
		},
	},
	Action: runMigrate,
}

func runMigrate(ctx context.Context, cmd *cli.Command) error {
	// Positional args after `migrate`: <goose-command> [args...]. Default to `up`.
	args := cmd.Args().Slice()
	gooseCmd := "up"
	if len(args) > 0 {
		gooseCmd = args[0]
		args = args[1:]
	}

	cfg, err := config.Load(cmd.String("config"))
	if err != nil {
		return err
	}
	driver := cfg.Database.DriverName()

	var sqlDB *sql.DB
	if driver == "sqlite" {
		sqlDB, err = sql.Open("sqlite", cfg.Database.SQLiteDSN())
	} else {
		sqlDB, err = sql.Open("pgx", cfg.Database.DSN())
	}
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer sqlDB.Close()
	if driver == "sqlite" {
		// Pin to one connection (mirrors db.openSQLite). A NO-TRANSACTION
		// migration that toggles `PRAGMA foreign_keys` only holds on a single
		// connection — without this, goose could run its statements across
		// different pooled connections and a table-rebuild migration would fail.
		sqlDB.SetMaxOpenConns(1)
	}

	dialect := "postgres"
	if driver == "sqlite" {
		dialect = "sqlite3"
	}
	if err := goose.SetDialect(dialect); err != nil {
		return err
	}

	// `create` writes a new .sql file to the on-disk per-engine migrations dir
	// (relative to backend/ — the Makefile sets that CWD). Every other command
	// reads the migrations embedded in the binary, so it works regardless of CWD.
	dir := "."
	if gooseCmd == "create" {
		dir = "migrations/postgres"
		if driver == "sqlite" {
			dir = "migrations/sqlite"
		}
	} else {
		goose.SetBaseFS(migrations.FS(driver))
	}

	return goose.RunContext(ctx, gooseCmd, sqlDB, dir, args...)
}

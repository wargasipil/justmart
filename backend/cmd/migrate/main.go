package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/glebarez/go-sqlite" // registers the pure-Go "sqlite" driver
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/migrations"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: migrate <up|down|status|create|reset|version>")
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	cfg, err := config.Load("")
	if err != nil {
		log.Fatal(err)
	}

	driver := cfg.Database.DriverName()

	var sqlDB *sql.DB
	if driver == "sqlite" {
		sqlDB, err = sql.Open("sqlite", cfg.Database.SQLiteDSN())
	} else {
		sqlDB, err = sql.Open("pgx", cfg.Database.DSN())
	}
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()

	dialect := "postgres"
	if driver == "sqlite" {
		dialect = "sqlite3"
	}
	if err := goose.SetDialect(dialect); err != nil {
		log.Fatal(err)
	}

	// `create` writes a new .sql file to the on-disk per-engine migrations dir
	// (relative to backend/ — the Makefile sets that CWD). Every other command
	// reads the migrations embedded in the binary, so it works regardless of CWD.
	dir := "."
	if cmd == "create" {
		dir = "migrations/postgres"
		if driver == "sqlite" {
			dir = "migrations/sqlite"
		}
	} else {
		goose.SetBaseFS(migrations.FS(driver))
	}

	if err := goose.Run(cmd, sqlDB, dir, args...); err != nil {
		log.Fatal(err)
	}
}

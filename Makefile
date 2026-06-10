.PHONY: up down reset-devel-data generate tidy run test-unit test-unit-postgres test-unit-all test-e2e test-e2e-sqlite test-browser test-all \
        migrate-up migrate-down migrate-status migrate-create \
        web-install web \
        embed-web build dist-windows docker-build docker-up docker-down installer \
        portable-windows backup

# `go -C backend run ...` runs the binary with CWD = backend/, so we point
# JUSTMART_CONFIG at the repo-root config from there. Using `export` (a Make
# directive, not a shell command) so this works under any shell make picks
# on the host — POSIX sh, bash, or Windows cmd.exe.
export JUSTMART_CONFIG := ../config.yaml

GO_BACKEND := go -C backend

# --- Docker ------------------------------------------------------------------
up:
	docker compose up -d

down:
	docker compose down

# Wipe the dev DB and start fresh. Works in cmd.exe, PowerShell, and bash —
# no shell idioms. `down -v` removes the named volume (the cluster); `up -d
# --wait` blocks on the compose-defined healthcheck until Postgres accepts
# connections. The next `make run` auto-applies migrations and creates the
# bootstrap owner.
reset-devel-data:
	docker compose down -v
	docker compose up -d --wait

# --- Proto codegen -----------------------------------------------------------
generate:
	buf generate

# --- Backend (Go) ------------------------------------------------------------
tidy:
	go -C backend mod tidy

run:
	$(GO_BACKEND) run ./cmd/server

# --- Packaging (single self-contained binary) --------------------------------
# embed-web builds the SPA and copies it into the Go embed dir. `build` and
# `dist-windows` then compile a single binary with the UI + migrations embedded.
embed-web:
	npm --prefix frontend ci
	npm --prefix frontend run build
	rm -rf backend/internal/web/dist/assets
	cp -r frontend/dist/. backend/internal/web/dist/

# Native single binary -> dist/justmart (serves UI + /api + auto-migrates).
build: embed-web
	@mkdir -p dist
	$(GO_BACKEND) build -ldflags "-s -w" -o ../dist/justmart ./cmd/server

# Windows single binary -> dist/justmart.exe (input to the installer build).
# Pure-Go deps mean no CGO, so this cross-compiles from any host.
dist-windows: embed-web
	@mkdir -p dist
	GOOS=windows GOARCH=amd64 $(GO_BACKEND) build -ldflags "-s -w" -o ../dist/justmart.exe ./cmd/server

# --- Docker (production image + compose) --------------------------------------
docker-build:
	docker build -t justmart:latest .

docker-up:
	docker compose -f docker-compose.prod.yml up -d --build

docker-down:
	docker compose -f docker-compose.prod.yml down

# --- Windows installer -------------------------------------------------------
# Assembles the payload (exe + bundled Postgres + WinSW) and runs Inno Setup.
# Requires PowerShell + Inno Setup (ISCC) on PATH; see packaging/windows/.
installer: dist-windows
	powershell -ExecutionPolicy Bypass -File packaging/windows/build-windows.ps1

# --- Portable Windows distribution (SQLite, no installer) --------------------
# Builds dist/justmart-portable-<ver>/ + .zip: justmart.exe + a SQLite config.yaml
# + launcher + README. Unzip-and-run, no Postgres, no Inno Setup. The script
# builds the exe itself; pass -SkipExeBuild to reuse an existing dist/justmart.exe.
portable-windows:
	powershell -ExecutionPolicy Bypass -File packaging/windows/build-portable.ps1

# Co-located handler unit tests: each internal/service/<domain>/<rpc>_test.go
# calls its handler method directly (no HTTP) against a fresh, migrated, throwaway
# SQLite DB (one temp file per test, see internal/service/servicetest). Self-
# contained: no dev Postgres, no config.yaml, no server. `-count=1` skips the
# test cache. Safe to run in parallel (each test has its own DB file).
test-unit:
	$(GO_BACKEND) test ./internal/service/... -count=1

# The SAME co-located unit suite, run against the dev Postgres (run `make up`
# first). JUSTMART_TEST_DB_DRIVER=postgres makes servicetest give every test its
# own throwaway schema on the dev cluster — the test files are identical to the
# SQLite run. `-p 1 -parallel 4` bounds concurrency so the per-test CREATE/DROP
# SCHEMA churn (each schema ~40 tables) stays under PG's shared lock table on a
# default-tuned server. PG creds default to the docker-compose values; override
# via JUSTMART_DB_{HOST,PORT,USER,PASSWORD,NAME,SSLMODE}.
test-unit-postgres: export JUSTMART_TEST_DB_DRIVER := postgres
test-unit-postgres:
	$(GO_BACKEND) test ./internal/service/... -count=1 -p 1 -parallel 4

# Run the co-located unit suite against BOTH engines back-to-back (SQLite first
# — fastest, no dependency — so it fails fast on the cheapest engine). This is
# the HARD-RULE gate: every RPC's unit test must pass on both.
test-unit-all: test-unit test-unit-postgres

# End-to-end / integration tests (in-process httptest server + real dev Postgres).
# Test binaries run with CWD = backend/e2e/, so the JUSTMART_CONFIG path needs
# two `..` to reach the repo-root config.yaml.
# `-count=1` disables Go's test-result caching.
test-e2e: export JUSTMART_CONFIG := ../../config.yaml
test-e2e:
	$(GO_BACKEND) test ./e2e/... -v -count=1

# Same integration suite, but against a fresh on-disk SQLite database (the
# turnkey engine). Reuses config.yaml for the bootstrap owner/auth; env vars
# override just the DB. The file lives under dist/ (gitignored) and is wiped
# each run so auto-migrate re-seeds the consolidated sqlite/00001_init.sql.
test-e2e-sqlite: export JUSTMART_CONFIG := ../../config.yaml
test-e2e-sqlite: export JUSTMART_DB_DRIVER := sqlite
test-e2e-sqlite: export JUSTMART_DB_PATH := $(CURDIR)/dist/justmart_test.sqlite
test-e2e-sqlite:
	@mkdir -p dist
	rm -f "$(CURDIR)/dist/justmart_test.sqlite" "$(CURDIR)/dist/justmart_test.sqlite-wal" "$(CURDIR)/dist/justmart_test.sqlite-shm"
	$(GO_BACKEND) test ./e2e/... -v -count=1

migrate-up:
	$(GO_BACKEND) run ./cmd/migrate up

migrate-down:
	$(GO_BACKEND) run ./cmd/migrate down

migrate-status:
	$(GO_BACKEND) run ./cmd/migrate status

# Usage: make migrate-create name=add_medicines_table
migrate-create:
	$(GO_BACKEND) run ./cmd/migrate create $(name) sql

# --- Frontend (React + Vite) -------------------------------------------------
web-install:
	npm --prefix frontend install

web:
	npm --prefix frontend run dev

# Browser E2E tests (Playwright). Assumes `make run` and `make web` are
# already running in separate terminals; tests hit http://localhost:5173 and
# share the dev DB. Suite runs against Chromium headless by default.
# (cd into frontend so playwright.config.ts is loaded relative to CWD.)
test-browser:
	cd frontend && npx playwright test

# Convenience: co-located unit suite on BOTH engines, then Go integration, then
# browser E2E. test-unit-all runs first (SQLite then Postgres), failing fast on
# the cheapest engine.
test-all: test-unit-all test-e2e test-browser

# --- Backups -----------------------------------------------------------------
# Snapshot the running Postgres into backups/backup_<timestamp>/.
# Produces the same layout as BackupService (database.sql.gz + manifest.txt)
# so CLI and in-app backups are interchangeable. Uses pg_dump from the
# docker-compose db container so no host pg_dump is required.
# Wire to cron for nightly backups in production.
backup:
	@stamp=$$(date +%Y-%m-%d_%H%M%S); \
	dir=backups/backup_$$stamp; \
	mkdir -p $$dir; \
	docker compose exec -T db pg_dump -U justmart justmart | gzip > $$dir/database.sql.gz; \
	size=$$(wc -c < $$dir/database.sql.gz | tr -d ' '); \
	ver=$$(docker compose exec -T db psql -U justmart -d justmart -tA -c "SELECT COALESCE(MAX(version_id),0) FROM goose_db_version WHERE is_applied" | tr -d '\r '); \
	dbver=$$(docker compose exec -T db psql -U justmart -d justmart -tA -c "SELECT version()" | tr -d '\r'); \
	{ \
	  echo "created_at=$$(date +%s)"; \
	  echo "created_at_iso=$$(date -u +%Y-%m-%dT%H:%M:%SZ)"; \
	  echo "app_version=dev"; \
	  echo "db_version=$$dbver"; \
	  echo "schema_version=$$ver"; \
	  echo "size_bytes=$$size"; \
	} > $$dir/manifest.txt; \
	echo "Wrote $$dir/"

.PHONY: up down reset-devel-data generate tidy wire run dev test-unit test-unit-postgres test-unit-all test-e2e test-e2e-sqlite test-browser test-all \
        migrate-up migrate-down migrate-status migrate-create \
        web-install web \
        embed-web build dist-windows dist-connector-windows docker-build docker-up docker-down installer \
        portable-windows backup faq-video \
        release-demo release-seed release-video release-encode release-all \
        graph graph-full graph-label graph-viz \
        fly-app fly-volume fly-secrets fly-setup fly-deploy fly-status fly-logs fly-ssh

# `go -C backend run ...` runs the binary with CWD = backend/, so we point
# JUSTMART_CONFIG at the repo-root config from there. Using `export` (a Make
# directive, not a shell command) so this works under any shell make picks
# on the host — POSIX sh, bash, or Windows cmd.exe.
export JUSTMART_CONFIG := ../config.yaml

GO_BACKEND := go -C backend

# Build version stamped into the binary via -ldflags -X main.version. Override on
# the CLI (e.g. `make build VERSION=1.2.0`); defaults to "dev". Drives the
# autoupdater's current-version display.
VERSION ?= dev
GO_LDFLAGS := -s -w -X main.version=$(VERSION)

# Model the graphify semantic pass drives through the local `claude` CLI.
# Override per-run, e.g. `make graph-full GRAPHIFY_MODEL=haiku` for a cheaper pass.
GRAPHIFY_MODEL ?= sonnet

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

# Regenerate the dependency-injection graph for cmd/server (google/wire). Run
# after changing backend/cmd/server/providers.go or wire.go; wire_gen.go is
# generated + committed. The wire CLI is a `tool` dependency in backend/go.mod,
# so no separate install step is needed.
wire:
	$(GO_BACKEND) tool wire ./cmd/server

run:
	$(GO_BACKEND) run ./cmd/server

# --- Development (live preview) ----------------------------------------------
# One command for the whole dev loop: the Go API (config.yaml `server.port`,
# 8089 locally) plus the Vite dev server with HMR, which proxies /api -> the
# backend (see frontend/vite.config.ts). Open the Vite URL, not :8089 — the
# `go run` backend serves the *embedded* SPA (a stale build, or the "frontend
# not built" stub on a fresh checkout), so live preview only comes from Vite.
#
# Implemented as a recursive `make -j2` over the existing `run` + `web` targets
# rather than shell `&`/`wait`, so it works under any shell make picks on the
# host (sh, bash, cmd.exe) — same reason JUSTMART_CONFIG is exported as a Make
# directive above. Ctrl-C stops both; if a server ever outlives it, find the pid
# with `netstat -ano | findstr LISTENING` and taskkill it.
#
# The order-only prerequisite installs frontend deps on a fresh checkout and is
# skipped once node_modules exists. $(MAKE) is quoted because the mingw make on
# a Windows host can live under a path with a space (C:\Users\First Last\...),
# which make would otherwise hand to CreateProcess unquoted and split.
dev: | frontend/node_modules
	@echo "backend  -> http://localhost:8089/api"
	@echo "frontend -> http://localhost:5175   <- open this one"
	@"$(MAKE)" -j2 --no-print-directory run web

frontend/node_modules:
	npm --prefix frontend install

# --- Knowledge graph (graphify) ----------------------------------------------
# Builds graphify-out/ (graph.json + GRAPH_REPORT.md + graph.html), which the
# coding agent queries instead of grepping. graphify-out/ is gitignored — it is
# per-machine, rebuilt, never committed. See docs/graphify.md.
graph: ## Refresh the knowledge graph after code changes (AST-only, no LLM cost)
	graphify update .

graph-full: export GRAPHIFY_CLAUDE_CLI_MODEL = $(GRAPHIFY_MODEL)
graph-full: ## Rebuild the graph incl. semantic extraction (local claude CLI, no API key)
	graphify extract . --backend claude-cli

graph-label: export GRAPHIFY_CLAUDE_CLI_MODEL = $(GRAPHIFY_MODEL)
graph-label: ## Re-name graph communities + regenerate GRAPH_REPORT.md
	graphify label . --backend claude-cli

# graphify skips its force-directed graph.html above 5000 nodes (this repo is
# ~12k), so the browsable viz here is the collapsible tree instead.
graph-viz: ## Emit graphify-out/GRAPH_TREE.html (browsable code tree)
	graphify tree --label justmart

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
	$(GO_BACKEND) build -ldflags "$(GO_LDFLAGS)" -o ../dist/justmart ./cmd/server

# Windows single binary -> dist/justmart.exe (input to the installer build).
# Pure-Go deps mean no CGO, so this cross-compiles from any host.
dist-windows: embed-web
	@mkdir -p dist
	GOOS=windows GOARCH=amd64 $(GO_BACKEND) build -ldflags "$(GO_LDFLAGS)" -o ../dist/justmart.exe ./cmd/server

# Cross-compile the standalone Windows print connector (no embedded UI). Ships
# as a small zip the shop runs next to the printer. The Windows-only spooler dep
# is isolated behind //go:build windows, so this is the only target that links it.
dist-connector-windows:
	@mkdir -p dist/connector
	GOOS=windows GOARCH=amd64 $(GO_BACKEND) build -ldflags "-s -w" -o ../dist/connector/justmart-connector.exe ./cmd/connector
	cp backend/cmd/connector/config.yaml.example dist/connector/
	cp backend/cmd/connector/start.bat dist/connector/
	cp backend/cmd/connector/start.ps1 dist/connector/

# --- Docker (production image + compose) --------------------------------------
docker-build:
	docker build -t justmart:latest .

docker-up:
	docker compose -f docker-compose.prod.yml up -d --build

docker-down:
	docker compose -f docker-compose.prod.yml down

# --- Fly.io (cloud deploy) ----------------------------------------------------
# One machine, SQLite on a persistent volume, printing disabled. Shape + caveats
# are documented in fly.toml and the Fly.io section of DEPLOYMENT.md.
#
# Only FLY_APP duplicates fly.toml (`fly apps create` needs the name before a
# config exists); every other target lets flyctl read the app from fly.toml in
# CWD. Rename in fly.toml -> pass FLY_APP=<newname> or edit it here too.
FLY_APP         ?= justmart
FLY_REGION      ?= sin
FLY_VOLUME      ?= justmart_data
# Size in GB. 1 = Fly's minimum, plenty for a SQLite shop DB + backups. A volume
# can be EXTENDED later (`fly volume extend <id> -s <gb>`) but never shrunk, so
# start small.
FLY_VOLUME_SIZE ?= 1

# One-time provisioning. Each step is a no-op when it already exists, so the
# whole thing is safe to re-run:
#   make fly-setup OWNER_EMAIL=owner@yourshop.com OWNER_PASSWORD='<strong>'
fly-setup: fly-app fly-volume fly-secrets

fly-app:
	@fly status -a $(FLY_APP) >/dev/null 2>&1 \
	  && echo "app $(FLY_APP) already exists" \
	  || fly apps create $(FLY_APP)

# A Fly volume attaches to exactly ONE machine and the stock ledger lives in
# SQLite, so this stays a single volume on purpose (--yes skips flyctl's
# "you're creating a single volume" redundancy prompt). Never scale past 1.
fly-volume:
	@fly volumes list -a $(FLY_APP) 2>/dev/null | grep -q "$(FLY_VOLUME)" \
	  && echo "volume $(FLY_VOLUME) already exists" \
	  || fly volume create $(FLY_VOLUME) -a $(FLY_APP) --region $(FLY_REGION) --size $(FLY_VOLUME_SIZE) --yes

# JWT secret is minted once and then LEFT ALONE — rotating it invalidates every
# access token in the field. The owner credentials are re-applied on purpose:
# EnsureBootstrapOwner re-reads them every boot, and an EMPTY env var falls back
# to the baked config.docker.yaml defaults (owner@justmart.local /
# change-me-on-first-login), so both must stay set permanently.
fly-secrets:
	@test -n "$(OWNER_EMAIL)"    || { echo "usage: make fly-secrets OWNER_EMAIL=owner@yourshop.com OWNER_PASSWORD='<strong-password>'"; exit 1; }
	@test -n "$(OWNER_PASSWORD)" || { echo "usage: make fly-secrets OWNER_EMAIL=owner@yourshop.com OWNER_PASSWORD='<strong-password>'"; exit 1; }
	@fly secrets list -a $(FLY_APP) 2>/dev/null | grep -q JUSTMART_JWT_SECRET \
	  && echo "JUSTMART_JWT_SECRET already set (not rotating)" \
	  || fly secrets set -a $(FLY_APP) JUSTMART_JWT_SECRET=$$(openssl rand -hex 32)
	fly secrets set -a $(FLY_APP) \
	  JUSTMART_OWNER_EMAIL='$(OWNER_EMAIL)' \
	  JUSTMART_OWNER_PASSWORD='$(OWNER_PASSWORD)'

# Build the image remotely and release it. The SPA + migrations are embedded by
# the Dockerfile and goose runs on boot (auto_migrate: true), so there is no
# separate build or migrate step. --ha=false keeps Fly from provisioning the
# standby machine it would normally add: the second machine cannot mount the
# volume, and two SQLite files would diverge.
fly-deploy:
	fly deploy -a $(FLY_APP) --ha=false

fly-status:
	fly status -a $(FLY_APP)

fly-logs:
	fly logs -a $(FLY_APP)

# Shell into the machine; the volume is at /data (SQLite DB + backups).
fly-ssh:
	fly ssh console -a $(FLY_APP)

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
#
# internal/printer rides along (it needs no DB at all): it renders the ESC/POS
# byte streams for receipts and barcode labels, and a wrong GS k frame produces
# a label that prints fine and scans as the wrong product — the kind of bug only
# a byte-level test catches.
test-unit: test-migrations
	$(GO_BACKEND) test ./internal/service/... ./internal/printer/... -count=1

# Migration guards. A committed migration has already run in production, so
# editing one desynchronises every existing database from a fresh install —
# twice already (repair migrations 00040 and 00052). These two tests make that
# impossible to land silently: TestMigrationsAreImmutable pins every migration's
# bytes, and TestSQLite_FrozenBaselineConvergesWithFresh proves a legacy DB
# upgraded in place reaches the same schema as a fresh one. SQLite-only and
# engine-independent, so they run once (not per-engine) and are cheap.
test-migrations:
	$(GO_BACKEND) test ./migrations/... ./internal/dbmigrate/... -count=1

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
	$(GO_BACKEND) run ./cmd/server migrate up

migrate-down:
	$(GO_BACKEND) run ./cmd/server migrate down

migrate-status:
	$(GO_BACKEND) run ./cmd/server migrate status

# Usage: make migrate-create name=add_medicines_table
migrate-create:
	$(GO_BACKEND) run ./cmd/server migrate create $(name) sql

# Pin newly added migrations in backend/migrations/checksums.txt. APPEND-ONLY on
# purpose: an already-pinned migration has run in production, so a changed hash
# is a bug (TestMigrationsAreImmutable), not something to regenerate away.
migrate-checksums:
	$(GO_BACKEND) run ./cmd/pinmigrations

# Convert qty-gated product discounts into grosir price tiers. Dry run (prints
# the plan, writes nothing); `make discount-to-grosir-apply` performs it.
# Both drop a CSV of the FULL report — converted and skipped — at $(csv);
# override with `make discount-to-grosir csv=somewhere/else.csv`. The path is
# rooted at the repo (GO_BACKEND runs with CWD=backend/, so a bare relative path
# would land there instead).
csv ?= dist/discount-to-grosir.csv

discount-to-grosir:
	@mkdir -p "$(dir $(CURDIR)/$(csv))"
	$(GO_BACKEND) run ./cmd/server discount-to-grosir --csv "$(CURDIR)/$(csv)"

discount-to-grosir-apply:
	@mkdir -p "$(dir $(CURDIR)/$(csv))"
	$(GO_BACKEND) run ./cmd/server discount-to-grosir --apply --csv "$(CURDIR)/$(csv)"

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

# --- FAQ tutorial videos -----------------------------------------------------
# Re-record the screen-capture tutorials in faq/videos/<question>/tutorial.webm
# by driving the real app (frontend/tests/faq/*.spec.ts). Deliberately NOT part
# of test-all: these assert nothing and take minutes each.
# Same prerequisites as test-browser (`make run` + `make web` up, dev DB), plus
# JUSTMART_TEST_OWNER_PASSWORD matching config.yaml bootstrap.owner_password.
# Narrow to one question with q=<slug substring>. See faq/README.md.
faq-video:
	cd frontend && npx playwright test -c playwright.faq.config.ts $(q)

# --- Release video -----------------------------------------------------------
# The YouTube product tour: release/video/justmart-product-tour-1080p.mp4.
#
# Unlike faq-video, this does NOT record against the dev stack. It drives a
# throwaway demo instance — its own binary, its own SQLite file, port 8099 —
# because the dev database is full of half-finished test rows and none of them
# belong in something published publicly. See release/README.md for the full
# walkthrough; the short version is:
#
#   make release-demo     # terminal 1: build + serve the demo instance
#   make release-seed     # once, into a fresh demo DB
#   make release-video    # record (~3.5 min)
#   make release-encode   # -> MP4 + youtube.md
#
# DEMO_DIR defaults to a scratch folder; override to keep the demo shop around.
DEMO_DIR ?= .release-demo
DEMO_URL ?= http://127.0.0.1:8099
DEMO_PASSWORD ?= demo12345

# Build the single binary and serve it against DEMO_DIR/demo.db. Runs in the
# foreground: leave it in its own terminal for the duration of a recording.
release-demo: build
	@mkdir -p $(DEMO_DIR)
	@test -f $(DEMO_DIR)/config.yaml || cp release/demo.config.yaml $(DEMO_DIR)/config.yaml
	cp dist/justmart $(DEMO_DIR)/justmart
	cd $(DEMO_DIR) && ./justmart

# Build the demo shop: catalog, grosir ladder, restock orders, and a month of
# backdated trading. Expects a FRESH demo.db — re-running over a seeded one
# fails on the uniqueness checks rather than doubling the catalog.
release-seed:
	node release/seed-demo.mjs --base $(DEMO_URL) --db $(DEMO_DIR)/demo.db --password $(DEMO_PASSWORD)

release-video:
	cd frontend && npx playwright test -c playwright.release.config.ts

# Encode the capture and build every upload asset: MP4, thumbnails, and the
# paste-ready title/description/tags under release/youtube/.
release-encode:
	node release/encode.mjs
	node release/thumbnail.mjs
	node release/description.mjs

release-all: release-video release-encode

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


download-tools: ## Download third-party dev tools into thirdparties/bin/
	powershell -ExecutionPolicy Bypass -File scripts/download-tools.ps1
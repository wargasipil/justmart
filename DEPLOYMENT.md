# Justmart — Deployment runbook

Single-shop deployment. Justmart ships as **one self-contained binary** that
serves the web UI and the API on a single port and auto-applies its database
migrations on boot. Two turnkey distribution flavors:

- **Docker image** — for a Linux box / VM / cloud host. `docker compose up`.
- **Windows installer** — for a pharmacy running everything on a Windows PC,
  optionally serving a few LAN registers. Double-click `JustmartSetup-*.exe`.

Both embed the SPA + migrations into the binary; there is **no separate nginx,
static host, or manual migrate step**.

---

## Flavor 1 — Docker

### Prerequisites
- Docker + Docker Compose v2.

### Deploy
```sh
cp .env.example .env          # set JUSTMART_JWT_SECRET (openssl rand -hex 32) + owner creds
docker compose -f docker-compose.prod.yml up -d --build
```
- The `app` image (built from the repo `Dockerfile`) serves UI + `/api` on `:8080`.
- `postgres` (pinned `postgres:18`) starts first; the app waits for it healthy,
  auto-migrates, then listens. Data persists in the `justmart-pgdata` named volume.
- Secrets come from `.env` (env overrides in `internal/config`): `JUSTMART_JWT_SECRET`,
  `JUSTMART_DB_PASSWORD`, `JUSTMART_OWNER_EMAIL`, `JUSTMART_OWNER_PASSWORD`, `JUSTMART_TZ`.

Browse `http://<host>:8080`, log in as the bootstrap owner, create real OWNER
users, then change/disable the bootstrap account.

### Update
```sh
git pull
docker compose -f docker-compose.prod.yml up -d --build   # rebuilds image, re-migrates on boot
```

### Make shortcuts
`make docker-build`, `make docker-up`, `make docker-down`.

---

## Flavor 2 — Windows installer

A self-contained `.exe` that installs the app, a **bundled PostgreSQL**, both as
auto-start Windows Services, plus a browser shortcut. Zero prerequisites for the
pharmacist. Full build + install + verification details: [packaging/windows/README.md](packaging/windows/README.md).

### Build (developer machine, Windows x64)
Needs Go 1.25+, Node 20+, and Inno Setup 6.
```powershell
make installer          # = dist-windows + packaging\windows\build-windows.ps1
# -> dist\JustmartSetup-<version>.exe
```

### Install (target PC)
Run the `.exe`. The wizard asks for the owner email/password, **network access**
(single-PC `127.0.0.1` vs LAN `0.0.0.0` + firewall rule), and ports. Post-install
initializes the DB, writes `C:\ProgramData\Justmart\config.yaml` (random
`jwt_secret` + DB password), registers + starts `justmart-postgres` and
`justmart-server`, and opens the browser.

### Topology
Editable later in `C:\ProgramData\Justmart\config.yaml` (`server.host`); run
`Restart-Service justmart-server` to apply. LAN mode adds a firewall rule for the
app port; PostgreSQL always stays bound to `127.0.0.1` (clients hit the app).

---

## Local dev (unchanged)
`make up` (Postgres) · `make run` (server on `:8080`) · `make web` (Vite on `:5173`,
proxies `/api` → `:8080`). Dev uses the Vite server, not the embedded SPA.

`make build` produces the native self-contained binary at `dist/justmart` for a
local production smoke test.

---

## Daily operations
| Action | Docker | Windows |
|---|---|---|
| Logs | `docker logs -f justmart-app` (JSON via `slog`) | `C:\ProgramData\Justmart\logs\` (WinSW) + Event Viewer |
| Migrations | automatic on boot | automatic on boot |
| Backup (one-off) | OWNER → **Settings → Backups → Create**, OR `make backup` (dev compose), OR `docker compose -f docker-compose.prod.yml exec justmart-postgres-prod pg_dump ...` | OWNER → **Settings → Backups → Create**, OR `scripts\justmart-backup.bat` → `C:\ProgramData\Justmart\backups\` |
| Backup (nightly) | cron `make backup` (writes the same per-timestamp dir as the in-app button) | Task Scheduler → `justmart-backup.bat` |
| Rotate JWT secret | edit `.env`, `up -d` | edit `config.yaml`, `Restart-Service justmart-server` (invalidates sessions) |
| Reset a password | OWNER → Users → "Issue reset token" → hand token OOB → user redeems at `/reset?token=...` | same |

## Database engine (Postgres vs SQLite)
- The same binary runs on either engine, selected by `database.driver` in `config.yaml` (or `JUSTMART_DB_DRIVER`): **`postgres`** (default — the Docker / multi-user deploy documented here) or **`sqlite`** (a turnkey, zero-dependency flavor — no Postgres, no Docker; set `database.path`, e.g. `./justmart.db`, or `JUSTMART_DB_PATH`). SQLite uses a pure-Go driver, auto-migrates the same schema on boot, and is correct under concurrency via a single-writer connection. Pick SQLite for a single-PC shop that doesn't want to run Postgres; pick Postgres for anything multi-user or networked.

## Backups
- **Layout (one folder per backup)** — every backup is its own per-timestamp directory under `backup.directory` (Docker: `/var/lib/justmart/backups` mounted as the `justmart-backups` named volume; Windows: `C:\ProgramData\Justmart\backups\`; dev: `./backups`):
  ```
  backup_2026-05-26_152400/
    database.sql.gz   (Postgres: pg_dump gzip; Windows .bat: plain database.sql)
    database.sqlite   (SQLite engine: VACUUM INTO snapshot — replaces database.sql.gz)
    manifest.txt      (created_at, schema_version, size_bytes, app/db version)
  ```
  Override the root with the `JUSTMART_BACKUP_DIR` env var or the `backup.directory` config key.
- **Three entry points produce identical layouts** so the OWNER's in-app list and the cron job's output are the same files:
  - **In-app** (OWNER → Settings → Backups → Create) — `BackupService.CreateBackup`; same screen lists past backups + a Delete with confirm. Refreshes every 60s.
  - **Docker / dev CLI** — `make backup` (uses the compose Postgres' bundled `pg_dump`).
  - **Windows CLI** — `C:\Program Files\Justmart\scripts\justmart-backup.bat` (uses the bundled `pg_dump.exe`).
- **Restore is manual.** A maintenance-mode UX is deferred, so there's no in-app restore button. Choose by flavor:
  - **Docker (compressed)**: stop the app, restore, restart.
    ```sh
    docker compose -f docker-compose.prod.yml stop app
    gunzip < /var/lib/docker/volumes/justmart_justmart-backups/_data/backup_<TS>/database.sql.gz \
      | docker compose -f docker-compose.prod.yml exec -T postgres psql -U justmart -d justmart
    docker compose -f docker-compose.prod.yml start app
    ```
  - **Windows (uncompressed)**: stop the service, restore, restart.
    ```powershell
    Stop-Service justmart-server
    & "$env:ProgramFiles\Justmart\pgsql\bin\psql.exe" -h 127.0.0.1 -p <port> -U justmart -d justmart `
      -f "$env:ProgramData\Justmart\backups\backup_<TS>\database.sql"
    Start-Service justmart-server
    ```
  The Postgres dump is `pg_dump --clean --if-exists`, so it drops existing tables before reloading — the live DB is safe to restore over, but make sure no other clients are writing while it runs.
  - **SQLite**: the dump is a complete copy of the database file. Stop the app, replace the live DB file, restart:
    ```sh
    # stop the server first, then:
    cp backups/backup_<TS>/database.sqlite ./justmart.db   # = your database.path
    # (remove any stale ./justmart.db-wal / -shm sidecars), then start the server
    ```
- **Operational discipline**: keep at least 7 nightlies; ship the latest off-host weekly (rsync / S3 / external drive). Test restore against a fresh DB before you rely on a backup.
- **Why the Docker image is no longer distroless**: `BackupService` subprocesses `pg_dump`, which doesn't ship in `distroless/static`. The runtime base switched to `debian:bookworm-slim` + `postgresql-client` (~80 MB heavier). The container still runs as a non-root UID (65532).
- **pg_dump auto-resolution**: every in-app `Create backup` looks for pg_dump in this order — system PATH → bundled next to the justmart binary (`<install>/pgsql/bin/pg_dump.exe`, the Windows installer layout — works even though the installer doesn't put that dir on PATH) → cache at `backup.pg_tools_dir` (default `%LOCALAPPDATA%\justmart\pgtools` on Windows / `~/.cache/justmart/pgtools` on Linux) → on **Windows only**, auto-download the EDB binaries zip (~75 MB) into the cache and reuse it. Linux without `postgresql-client` (and outside Docker) gets a clear "install postgresql-client" error instead of a download — apt is the right answer there. Override the cache root with `JUSTMART_PG_TOOLS_DIR`.
- **First-time Create cost on Windows dev**: the EDB zip download is **~75 MB** and has been observed taking **30+ min on slow foreign network links** before the remote occasionally drops the connection. The resolver supports **HTTP Range resume** (partial `.tmp` files survive failures) and retries up to 3 times in one Create call, so a click eventually succeeds even on flaky links. Subsequent backups use the cached binary and complete in under a second.
- **Manual escape hatch** (recommended when the auto-download is too slow): drop a `pg_dump.exe` (any recent version from a PostgreSQL Windows install) at `%LOCALAPPDATA%\justmart\pgtools\pgsql\bin\pg_dump.exe`. The resolver's step 3 (cache lookup) finds it and skips the download entirely.

## Observability
- Structured JSON logs via `log/slog`.
- Every write RPC is recorded in the `audit_log` table (`internal/auth/audit.go`):
  ```sql
  SELECT created_at, user_id, role, procedure, ok, code, message
  FROM audit_log WHERE created_at > now() - interval '24 hours' ORDER BY created_at DESC;
  ```
- No external metrics endpoint today; add a Prometheus `/metrics` in a follow-up.

## Security checklist
- Set a strong `auth.jwt_secret` (Docker: `JUSTMART_JWT_SECRET`; Windows: generated automatically).
- Change the bootstrap owner password after first login; disable the bootstrap account.
- Put TLS in front for any non-localhost exposure (the server speaks plaintext HTTP).
- PostgreSQL is reachable only from the app (no published host port in prod compose; localhost-only on Windows).
- Login rate limit: 5 attempts then ~1/min refill per email (in-process, single-node).
- Refresh tokens rotate on every Refresh; replaying a revoked token revokes the whole family.

## Multi-branch
- Supported at the data layer (Phase 8); `branches` is seeded with `MAIN`. Create more via `/branches` (OWNER).
- Frontend sends `X-Branch-Id` per RPC; switching branches reloads. Per-list branch filtering is opt-in per RPC (see CLAUDE.md "Known gaps").

## Known limitations
- ESC/POS dispatch assumes backend + printer share a LAN (raw TCP :9100).
- BPJS / e-Faktur are local stubs pending real credentials.
- No SMTP password-reset (OWNER-issued token, handed OOB).
- No HA: single-process, in-memory rate limiter; async audit-write loses queued rows on crash.
- Windows + Docker images are **unsigned**; expect SmartScreen on the installer until code-signed.

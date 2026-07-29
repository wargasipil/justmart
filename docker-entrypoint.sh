#!/bin/sh
# Justmart container entrypoint.
#
# Purpose: make mounted volumes writable by the unprivileged runtime user.
# Fly.io (and a fresh Docker named volume) mounts a volume owned by root, but
# the app runs as uid 65532 — so without this it cannot create its SQLite DB
# (JUSTMART_DB_PATH, e.g. /data/justmart.db) or write backups.
#
# So: if we start as root, take ownership of the writable dirs, then DROP to
# 65532 and exec the server. The process the app runs as is unchanged (65532),
# so the non-root posture is preserved — root exists only for the chown.
# If we're already unprivileged (e.g. `docker run --user`), just exec.
#
# setpriv ships in util-linux, which is part of the debian:bookworm-slim base —
# no extra package needed.
set -e

APP=/app/justmart
UID_GID=65532:65532

if [ "$(id -u)" = "0" ]; then
    # /data      -> Fly volume (SQLite DB + backups)
    # /var/lib/justmart/backups -> compose named volume (backups)
    for dir in /data /var/lib/justmart/backups; do
        if [ -d "$dir" ]; then
            chown -R "$UID_GID" "$dir" 2>/dev/null || true
        fi
    done
    exec setpriv --reuid=65532 --regid=65532 --clear-groups "$APP" "$@"
fi

exec "$APP" "$@"

-- +goose NO TRANSACTION
-- Mirror of postgres 00035_user_role_apoteker.sql — widen users.role CHECK to
-- include APOTEKER. SQLite can't ALTER a CHECK constraint, so rebuild the table
-- (create-copy-drop-rename). PRAGMA foreign_keys can't change inside a tx, hence
-- NO TRANSACTION; foreign_keys is toggled OFF around the drop/rename so the
-- tables that reference users (sales, refresh_tokens, …) don't block it — row
-- ids are preserved, so those FK references stay valid.

-- +goose Up
PRAGMA foreign_keys=OFF;
-- +goose StatementBegin
CREATE TABLE users_new (
    id            TEXT PRIMARY KEY NOT NULL,
    email         TEXT COLLATE NOCASE NOT NULL UNIQUE,
    name          TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('OWNER','PHARMACIST','CASHIER','APOTEKER')),
    active        INTEGER NOT NULL DEFAULT 1,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);
-- +goose StatementEnd
INSERT INTO users_new (id, email, name, password_hash, role, active, created_at, updated_at)
    SELECT id, email, name, password_hash, role, active, created_at, updated_at FROM users;
DROP TABLE users;
ALTER TABLE users_new RENAME TO users;
CREATE INDEX users_role_idx ON users(role);
PRAGMA foreign_keys=ON;

-- +goose Down
PRAGMA foreign_keys=OFF;
-- +goose StatementBegin
CREATE TABLE users_old (
    id            TEXT PRIMARY KEY NOT NULL,
    email         TEXT COLLATE NOCASE NOT NULL UNIQUE,
    name          TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('OWNER','PHARMACIST','CASHIER')),
    active        INTEGER NOT NULL DEFAULT 1,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);
-- +goose StatementEnd
INSERT INTO users_old (id, email, name, password_hash, role, active, created_at, updated_at)
    SELECT id, email, name, password_hash, role, active, created_at, updated_at FROM users;
DROP TABLE users;
ALTER TABLE users_old RENAME TO users;
CREATE INDEX users_role_idx ON users(role);
PRAGMA foreign_keys=ON;

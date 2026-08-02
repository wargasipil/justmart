-- +goose NO TRANSACTION
-- Mirror of postgres 00053_user_role_waiter.sql — widen users.role CHECK to
-- include WAITER. SQLite can't ALTER a CHECK constraint, so rebuild the table
-- (create-copy-drop-rename), exactly as 00035 did for APOTEKER. PRAGMA
-- foreign_keys can't change inside a tx, hence NO TRANSACTION; it is toggled OFF
-- around the drop/rename so the tables referencing users (sales, refresh_tokens,
-- user_avatars, …) don't block it — row ids are preserved, so those FK
-- references stay valid.
--
-- The column list is 00035's shape PLUS avatar_updated_at (added by 00050). A
-- rebuild must carry every column the table has by now, or the copy silently
-- drops data.

-- +goose Up
PRAGMA foreign_keys=OFF;
-- +goose StatementBegin
CREATE TABLE users_new (
    id                TEXT PRIMARY KEY NOT NULL,
    email             TEXT COLLATE NOCASE NOT NULL UNIQUE,
    name              TEXT NOT NULL DEFAULT '',
    password_hash     TEXT NOT NULL,
    role              TEXT NOT NULL CHECK (role IN ('OWNER','PHARMACIST','CASHIER','APOTEKER','WAITER')),
    active            INTEGER NOT NULL DEFAULT 1,
    created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    avatar_updated_at DATETIME
);
-- +goose StatementEnd
INSERT INTO users_new (id, email, name, password_hash, role, active, created_at, updated_at, avatar_updated_at)
    SELECT id, email, name, password_hash, role, active, created_at, updated_at, avatar_updated_at FROM users;
DROP TABLE users;
ALTER TABLE users_new RENAME TO users;
CREATE INDEX users_role_idx ON users(role);
PRAGMA foreign_keys=ON;

-- +goose Down
PRAGMA foreign_keys=OFF;
-- +goose StatementBegin
CREATE TABLE users_old (
    id                TEXT PRIMARY KEY NOT NULL,
    email             TEXT COLLATE NOCASE NOT NULL UNIQUE,
    name              TEXT NOT NULL DEFAULT '',
    password_hash     TEXT NOT NULL,
    role              TEXT NOT NULL CHECK (role IN ('OWNER','PHARMACIST','CASHIER','APOTEKER')),
    active            INTEGER NOT NULL DEFAULT 1,
    created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    avatar_updated_at DATETIME
);
-- +goose StatementEnd
INSERT INTO users_old (id, email, name, password_hash, role, active, created_at, updated_at, avatar_updated_at)
    SELECT id, email, name, password_hash, role, active, created_at, updated_at, avatar_updated_at FROM users;
DROP TABLE users;
ALTER TABLE users_old RENAME TO users;
CREATE INDEX users_role_idx ON users(role);
PRAGMA foreign_keys=ON;

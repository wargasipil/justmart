-- +goose Up
-- Mirror of postgres 00050_user_avatars.sql (SQLite dialect). UUID->TEXT (the
-- user_id is supplied by the caller, not generated), BYTEA->BLOB,
-- TIMESTAMPTZ->DATETIME. Two renditions per upload (original + thumb) — see the
-- postgres file for why they are separate columns and why thumb_data is NOT NULL.
--
-- No table rebuild needed: the new table is brand new, and the users column is
-- an additive NULLable ADD COLUMN, which SQLite's ALTER TABLE supports.
CREATE TABLE user_avatars (
    user_id      TEXT PRIMARY KEY NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content_type TEXT NOT NULL,
    image_data   BLOB NOT NULL,
    thumb_data   BLOB NOT NULL,
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

ALTER TABLE users ADD COLUMN avatar_updated_at DATETIME;

-- +goose Down
ALTER TABLE users DROP COLUMN avatar_updated_at;
DROP TABLE IF EXISTS user_avatars;

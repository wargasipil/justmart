-- +goose Up
-- Mirror of postgres 00063_batch_expiry_changes.sql (SQLite dialect). See that
-- file for why expiry corrections get their own history rather than relying on
-- audit_log. UUID->TEXT (filled by the Go create-callback), TIMESTAMPTZ->DATETIME.
CREATE TABLE batch_expiry_changes (
    id                TEXT PRIMARY KEY NOT NULL,
    batch_id          TEXT NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    old_expiry_date   DATE NOT NULL,
    old_expiry_source TEXT NOT NULL,
    new_expiry_date   DATE NOT NULL,
    new_expiry_source TEXT NOT NULL,
    reason            TEXT NOT NULL,
    changed_by        TEXT NOT NULL REFERENCES users(id),
    changed_at        DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX batch_expiry_changes_batch_idx
  ON batch_expiry_changes(batch_id, changed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS batch_expiry_changes;

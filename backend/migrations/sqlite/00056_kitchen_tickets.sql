-- +goose Up
-- Mirror of postgres 00056_kitchen_tickets.sql (SQLite dialect). See the
-- postgres file for why firing is per-line and why the note is free text.
-- TIMESTAMPTZ->DATETIME. No table rebuild: both adds are additive (one NULLable,
-- one NOT NULL with a constant default), which SQLite's ALTER TABLE ADD COLUMN
-- supports.
ALTER TABLE sale_items ADD COLUMN fired_at DATETIME;
ALTER TABLE sale_items ADD COLUMN kitchen_note TEXT NOT NULL DEFAULT '';

CREATE INDEX sale_items_unfired_idx ON sale_items(sale_id) WHERE fired_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS sale_items_unfired_idx;
ALTER TABLE sale_items DROP COLUMN kitchen_note;
ALTER TABLE sale_items DROP COLUMN fired_at;

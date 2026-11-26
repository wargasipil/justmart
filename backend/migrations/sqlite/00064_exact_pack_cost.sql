-- +goose Up
-- Mirror of postgres 00064_exact_pack_cost.sql (SQLite dialect). See that file
-- for what the three columns mean. Plain ADD COLUMNs with defaults: no CHECK,
-- no table rebuild, and every existing row reads 0 = "not recorded", which is
-- the legacy path on both columns' readers.
ALTER TABLE purchase_order_items ADD COLUMN pack_cost_price INTEGER NOT NULL DEFAULT 0;
ALTER TABLE batches ADD COLUMN cost_pack_price INTEGER NOT NULL DEFAULT 0;
ALTER TABLE batches ADD COLUMN cost_pack_factor INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE batches DROP COLUMN cost_pack_factor;
ALTER TABLE batches DROP COLUMN cost_pack_price;
ALTER TABLE purchase_order_items DROP COLUMN pack_cost_price;

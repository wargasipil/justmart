-- +goose Up
-- Mirror of postgres 00038 (SQLite dialect). Plain ADD COLUMN with constant
-- defaults — tx-safe, no table rebuild.
ALTER TABLE sale_items ADD COLUMN discount_type       TEXT    NOT NULL DEFAULT 'FIXED';
ALTER TABLE sale_items ADD COLUMN discount_value      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sales      ADD COLUMN cart_discount_type  TEXT    NOT NULL DEFAULT 'FIXED';
ALTER TABLE sales      ADD COLUMN cart_discount_value INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE sales      DROP COLUMN cart_discount_value;
ALTER TABLE sales      DROP COLUMN cart_discount_type;
ALTER TABLE sale_items DROP COLUMN discount_value;
ALTER TABLE sale_items DROP COLUMN discount_type;

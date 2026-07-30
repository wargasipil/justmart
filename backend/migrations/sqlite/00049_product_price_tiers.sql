-- +goose Up
-- Mirror of postgres 00049_product_price_tiers.sql (SQLite dialect). Grosir
-- (wholesale) quantity price tiers + the two sale_items price columns.
-- UUID->TEXT (PKs filled by the Go create-callback), BIGINT->INTEGER,
-- TIMESTAMPTZ->DATETIME. No table rebuild is needed: the CHECKs sit on a brand
-- new table (inline is free) and both sale_items adds are additive
-- NOT NULL DEFAULT <constant>, which SQLite's ALTER TABLE ADD COLUMN supports.
CREATE TABLE product_price_tiers (
    id              TEXT PRIMARY KEY NOT NULL,
    product_id      TEXT NOT NULL REFERENCES products(id),
    product_unit_id TEXT NOT NULL REFERENCES product_units(id),
    unit_name       TEXT    NOT NULL DEFAULT '',
    unit_factor     INTEGER NOT NULL DEFAULT 1,
    min_qty         INTEGER NOT NULL CHECK (min_qty >= 2),
    price           INTEGER NOT NULL DEFAULT 0 CHECK (price >= 0),
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX product_price_tiers_unit_min_qty_key ON product_price_tiers(product_unit_id, min_qty);
CREATE INDEX product_price_tiers_product_idx ON product_price_tiers(product_id);

ALTER TABLE sale_items ADD COLUMN list_price_snapshot INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sale_items ADD COLUMN tier_min_qty        INTEGER NOT NULL DEFAULT 0;
-- Backfill: before grosir, unit_price_snapshot WAS the catalog price.
UPDATE sale_items SET list_price_snapshot = unit_price_snapshot;

-- +goose Down
ALTER TABLE sale_items DROP COLUMN tier_min_qty;
ALTER TABLE sale_items DROP COLUMN list_price_snapshot;
DROP TABLE IF EXISTS product_price_tiers;

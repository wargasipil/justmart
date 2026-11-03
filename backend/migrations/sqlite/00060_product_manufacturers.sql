-- +goose Up
-- Mirror of postgres 00060_product_manufacturers.sql (SQLite dialect). See that
-- file for why the approved-source LIST lives on the product, the FACT lives on
-- the lot, and products.manufacturer_id stays as the primary pointer.
-- UUID->TEXT (PKs filled by the Go create-callback), TIMESTAMPTZ->DATETIME.
-- No table rebuild needed: one new table plus two plain nullable columns, which
-- SQLite's ALTER TABLE ADD COLUMN supports (a REFERENCES clause is allowed as
-- long as the new column defaults to NULL, which these do).
CREATE TABLE product_manufacturers (
    id              TEXT PRIMARY KEY NOT NULL,
    product_id      TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    manufacturer_id TEXT NOT NULL REFERENCES manufacturers(id),
    created_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX product_manufacturers_pair_idx
  ON product_manufacturers(product_id, manufacturer_id);
CREATE INDEX product_manufacturers_manufacturer_idx
  ON product_manufacturers(manufacturer_id);

-- Backfill. lower(hex(randomblob(16))) rather than a UUID function: SQLite has
-- none, and this runs in SQL where the Go create-callback that normally fills a
-- PK never sees the row. The shape does not matter to any reader -- nothing
-- joins on product_manufacturers.id, it exists only to give the row a key.
INSERT INTO product_manufacturers (id, product_id, manufacturer_id)
SELECT lower(hex(randomblob(16))), id, manufacturer_id
FROM products WHERE manufacturer_id IS NOT NULL;

ALTER TABLE batches ADD COLUMN manufacturer_id TEXT REFERENCES manufacturers(id);
CREATE INDEX batches_manufacturer_idx
  ON batches(manufacturer_id) WHERE manufacturer_id IS NOT NULL;

ALTER TABLE purchase_order_items ADD COLUMN manufacturer_id TEXT REFERENCES manufacturers(id);
CREATE INDEX purchase_order_items_manufacturer_idx
  ON purchase_order_items(manufacturer_id) WHERE manufacturer_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS purchase_order_items_manufacturer_idx;
ALTER TABLE purchase_order_items DROP COLUMN manufacturer_id;
DROP INDEX IF EXISTS batches_manufacturer_idx;
ALTER TABLE batches DROP COLUMN manufacturer_id;
DROP TABLE IF EXISTS product_manufacturers;

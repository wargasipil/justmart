-- +goose Up
-- Mirror of postgres 00058_product_tier_prices.sql (SQLite dialect). Grosir tier
-- price history, keyed by the RUNG (product_unit_id, min_qty) rather than the
-- hard-deleted tier row — see the postgres file for the reasoning.
-- UUID->TEXT (PKs filled by the Go create-callback), BIGINT->INTEGER,
-- TIMESTAMPTZ->DATETIME. No table rebuild: this is a brand new table, so the
-- CHECKs and the partial unique index are inline/free.
CREATE TABLE product_tier_prices (
    id              TEXT PRIMARY KEY NOT NULL,
    product_id      TEXT NOT NULL REFERENCES products(id),
    product_unit_id TEXT NOT NULL REFERENCES product_units(id),
    unit_name       TEXT    NOT NULL DEFAULT '',
    min_qty         INTEGER NOT NULL CHECK (min_qty >= 2),
    price           INTEGER NOT NULL CHECK (price >= 0),
    effective_from  DATETIME NOT NULL DEFAULT (datetime('now')),
    effective_to    DATETIME,
    changed_by      TEXT REFERENCES users(id),
    created_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX product_tier_prices_open_idx
  ON product_tier_prices(product_unit_id, min_qty)
  WHERE effective_to IS NULL;
CREATE INDEX product_tier_prices_product_idx ON product_tier_prices(product_id);

-- Backfill: one open row per existing tier at its current price (effective_from
-- = updated_at, when that price took effect). hex(randomblob(..)) builds the
-- UUID text here because the Go create-callback only fires on GORM inserts.
INSERT INTO product_tier_prices (id, product_id, product_unit_id, unit_name, min_qty, price, effective_from)
SELECT
  lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' ||
  substr(lower(hex(randomblob(2))), 2) || '-a' ||
  substr(lower(hex(randomblob(2))), 2) || '-' || lower(hex(randomblob(6))),
  product_id, product_unit_id, unit_name, min_qty, price, updated_at
FROM product_price_tiers;

-- +goose Down
DROP TABLE IF EXISTS product_tier_prices;

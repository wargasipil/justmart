-- +goose Up
-- Mirror of postgres 00046_product_discounts.sql (SQLite dialect). Drops the
-- standalone discount tables (00044) and adds per-product discounts + two
-- sale_items flags. UUID->TEXT, BIGINT/BOOLEAN->INTEGER, TIMESTAMPTZ->DATETIME.
DROP TABLE IF EXISTS discount_products;
DROP TABLE IF EXISTS discounts;

CREATE TABLE product_discounts (
    id            TEXT PRIMARY KEY NOT NULL,
    product_id    TEXT NOT NULL REFERENCES products(id),
    discount_type TEXT NOT NULL DEFAULT 'PERCENT' CHECK (discount_type IN ('FIXED', 'PERCENT')),
    per_item      INTEGER NOT NULL DEFAULT 0,
    value         INTEGER NOT NULL DEFAULT 0,
    min_qty       INTEGER NOT NULL DEFAULT 0,
    expires_at    DATE,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX product_discounts_product_idx ON product_discounts(product_id);

ALTER TABLE sale_items ADD COLUMN discount_per_item INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sale_items ADD COLUMN discount_manual   INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE sale_items DROP COLUMN discount_manual;
ALTER TABLE sale_items DROP COLUMN discount_per_item;
DROP TABLE IF EXISTS product_discounts;
CREATE TABLE discounts (
    id             TEXT PRIMARY KEY NOT NULL,
    name           TEXT NOT NULL,
    mode           TEXT NOT NULL CHECK (mode IN ('CODE', 'PRODUCT')),
    code           TEXT NOT NULL DEFAULT '',
    discount_type  TEXT NOT NULL DEFAULT 'PERCENT' CHECK (discount_type IN ('FIXED', 'PERCENT')),
    discount_value INTEGER NOT NULL DEFAULT 0,
    min_qty        INTEGER NOT NULL DEFAULT 0,
    expires_at     DATE,
    created_at     DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at     DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX discounts_code_key ON discounts(code) WHERE code <> '';
CREATE TABLE discount_products (
    discount_id TEXT NOT NULL REFERENCES discounts(id) ON DELETE CASCADE,
    product_id  TEXT NOT NULL REFERENCES products(id),
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (discount_id, product_id)
);
CREATE INDEX discount_products_product_idx ON discount_products(product_id);

-- +goose Up
-- Mirror of postgres 00044_discounts.sql (SQLite dialect). Incremental so an
-- existing DB upgrades in place. UUID->TEXT (PKs filled by the create-callback),
-- BIGINT->INTEGER, TIMESTAMPTZ->DATETIME.
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

-- +goose Down
DROP TABLE IF EXISTS discount_products;
DROP TABLE IF EXISTS discounts;

-- +goose Up
-- Mirror of postgres 00042_price_agreements.sql (SQLite dialect). Incremental so
-- an existing DB upgrades in place. UUID->TEXT (PKs filled by the create-callback),
-- TIMESTAMPTZ->DATETIME, BIGINT->INTEGER, BOOLEAN->INTEGER.
CREATE TABLE price_agreements (
    id              TEXT PRIMARY KEY NOT NULL,
    supplier_id     TEXT NOT NULL REFERENCES suppliers(id),
    product_id      TEXT NOT NULL REFERENCES products(id),
    product_unit_id TEXT NOT NULL REFERENCES product_units(id),
    unit_name       TEXT    NOT NULL DEFAULT '',
    unit_factor     INTEGER NOT NULL DEFAULT 1,
    price           INTEGER NOT NULL DEFAULT 0,
    valid_from      DATE,
    valid_until     DATE,
    note            TEXT    NOT NULL DEFAULT '',
    active          INTEGER NOT NULL DEFAULT 1,
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX price_agreements_active_key ON price_agreements(supplier_id, product_id, product_unit_id) WHERE active;
CREATE INDEX price_agreements_supplier_idx ON price_agreements(supplier_id) WHERE active;
CREATE INDEX price_agreements_product_idx  ON price_agreements(product_id) WHERE active;

-- +goose Down
DROP TABLE IF EXISTS price_agreements;

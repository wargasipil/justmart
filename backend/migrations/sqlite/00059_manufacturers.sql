-- +goose Up
-- Mirror of postgres 00059_manufacturers.sql (SQLite dialect). See that file for
-- why pabrik and pemasok are separate tables and why there are no bank columns.
-- UUID->TEXT (PKs filled by the Go create-callback), TIMESTAMPTZ->DATETIME.
-- No table rebuild needed: `manufacturers` is brand new, and products gains a
-- plain nullable column, which SQLite's ALTER TABLE ADD COLUMN supports.
CREATE TABLE manufacturers (
    id            TEXT PRIMARY KEY NOT NULL,
    code          TEXT NOT NULL,
    name          TEXT NOT NULL,
    address       TEXT NOT NULL DEFAULT '',
    phone         TEXT NOT NULL DEFAULT '',
    contact_email TEXT NOT NULL DEFAULT '',
    note          TEXT NOT NULL DEFAULT '',
    active        BOOLEAN NOT NULL DEFAULT 1,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX manufacturers_code_idx ON manufacturers(code);
CREATE UNIQUE INDEX manufacturers_name_active_idx ON manufacturers(name) WHERE active = 1;

ALTER TABLE products ADD COLUMN manufacturer_id TEXT REFERENCES manufacturers(id);
CREATE INDEX products_manufacturer_idx ON products(manufacturer_id) WHERE manufacturer_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS products_manufacturer_idx;
ALTER TABLE products DROP COLUMN manufacturer_id;
DROP TABLE IF EXISTS manufacturers;

-- +goose Up
-- Mirror of postgres 00055_dining_tables.sql (SQLite dialect). See the postgres
-- file for why a table belongs to a warehouse and why an open bill is just a
-- DRAFT sale. UUID->TEXT (PKs filled by the Go create-callback),
-- TIMESTAMPTZ->DATETIME, BOOLEAN->INTEGER.
--
-- No table rebuild: dining_tables is brand new (inline CHECKs are free there),
-- and all three sales adds are additive — the two NOT NULL ones carry constant
-- defaults, which SQLite's ALTER TABLE ADD COLUMN supports, and table_id is
-- NULLable, which is what lets it carry a REFERENCES clause.
CREATE TABLE dining_tables (
    id           TEXT PRIMARY KEY NOT NULL,
    warehouse_id TEXT NOT NULL REFERENCES warehouses(id),
    code         TEXT NOT NULL,
    name         TEXT NOT NULL DEFAULT '',
    area         TEXT NOT NULL DEFAULT '',
    seats        INTEGER NOT NULL DEFAULT 0 CHECK (seats >= 0),
    active       INTEGER NOT NULL DEFAULT 1,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX dining_tables_code_key ON dining_tables(warehouse_id, code) WHERE active;
CREATE INDEX dining_tables_warehouse_idx ON dining_tables(warehouse_id);

ALTER TABLE sales ADD COLUMN table_id TEXT REFERENCES dining_tables(id);
ALTER TABLE sales ADD COLUMN order_type TEXT NOT NULL DEFAULT '';
ALTER TABLE sales ADD COLUMN guest_count INTEGER NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX sales_open_table_idx ON sales(table_id)
  WHERE table_id IS NOT NULL AND status = 'DRAFT';
CREATE INDEX sales_table_idx ON sales(table_id);

-- +goose Down
DROP INDEX IF EXISTS sales_table_idx;
DROP INDEX IF EXISTS sales_open_table_idx;
ALTER TABLE sales DROP COLUMN guest_count;
ALTER TABLE sales DROP COLUMN order_type;
ALTER TABLE sales DROP COLUMN table_id;
DROP TABLE IF EXISTS dining_tables;

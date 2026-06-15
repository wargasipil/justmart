-- +goose Up
-- Mirror of postgres 00039_product_restocks.sql (SQLite dialect). Incremental so
-- an existing DB upgrades in place. UUID->TEXT (PKs filled by the create-callback),
-- TIMESTAMPTZ->DATETIME, BIGINT->INTEGER.
CREATE TABLE product_last_restocks (
    id                  TEXT PRIMARY KEY NOT NULL,
    warehouse_id        TEXT NOT NULL REFERENCES warehouses(id),
    product_id          TEXT NOT NULL REFERENCES products(id),
    supplier_id         TEXT NOT NULL REFERENCES suppliers(id),
    last_price          INTEGER NOT NULL DEFAULT 0,
    last_qty            INTEGER NOT NULL DEFAULT 0,
    last_discount_type  TEXT    NOT NULL DEFAULT 'FIXED',
    last_discount_value INTEGER NOT NULL DEFAULT 0,
    last_created_at     DATETIME NOT NULL,
    last_arrived_at     DATETIME NOT NULL,
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX product_last_restocks_key ON product_last_restocks(warehouse_id, product_id, supplier_id);
CREATE INDEX product_last_restocks_supplier_idx ON product_last_restocks(supplier_id, warehouse_id);
CREATE INDEX product_last_restocks_product_idx  ON product_last_restocks(product_id, warehouse_id);

CREATE TABLE product_restock_logs (
    id                 TEXT PRIMARY KEY NOT NULL,
    warehouse_id       TEXT NOT NULL REFERENCES warehouses(id),
    product_id         TEXT NOT NULL REFERENCES products(id),
    supplier_id        TEXT NOT NULL REFERENCES suppliers(id),
    price              INTEGER NOT NULL DEFAULT 0,
    qty                INTEGER NOT NULL DEFAULT 0,
    discount_type      TEXT    NOT NULL DEFAULT 'FIXED',
    discount_value     INTEGER NOT NULL DEFAULT 0,
    restock_created_at DATETIME NOT NULL,
    restock_arrived_at DATETIME NOT NULL,
    receipt_id         TEXT REFERENCES purchase_receipts(id),
    created_at         DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX product_restock_logs_product_idx  ON product_restock_logs(product_id, warehouse_id, restock_arrived_at);
CREATE INDEX product_restock_logs_supplier_idx ON product_restock_logs(supplier_id, warehouse_id);

-- +goose Down
DROP TABLE IF EXISTS product_restock_logs;
DROP TABLE IF EXISTS product_last_restocks;

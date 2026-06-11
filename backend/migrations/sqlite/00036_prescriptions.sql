-- +goose Up
-- Mirror of postgres 00036_prescriptions.sql (SQLite dialect). Incremental so an
-- existing DB (consolidated 00001 baseline) upgrades in place; goose applies this
-- only when version > 1.
CREATE TABLE rx_no_counters (year INTEGER PRIMARY KEY NOT NULL, last_seq INTEGER NOT NULL DEFAULT 0);

CREATE TABLE prescriptions (
    id          TEXT PRIMARY KEY NOT NULL,
    rx_no       TEXT UNIQUE,
    customer_id TEXT NOT NULL REFERENCES customers(id),
    issuer_name TEXT NOT NULL,
    issued_at   DATE NOT NULL,
    expires_at  DATE NOT NULL,
    note        TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','VOIDED')),
    created_by  TEXT NOT NULL REFERENCES users(id),
    branch_id   TEXT,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE prescription_items (
    id                  TEXT PRIMARY KEY NOT NULL,
    prescription_id     TEXT NOT NULL REFERENCES prescriptions(id) ON DELETE CASCADE,
    product_id          TEXT NOT NULL REFERENCES products(id),
    prescribed_qty      INTEGER NOT NULL CHECK (prescribed_qty > 0),
    dispensed_qty       INTEGER NOT NULL DEFAULT 0 CHECK (dispensed_qty >= 0),
    dosage_instructions TEXT NOT NULL DEFAULT '',
    note                TEXT NOT NULL DEFAULT '',
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    CHECK (dispensed_qty <= prescribed_qty)
);

CREATE INDEX prescriptions_customer_idx        ON prescriptions(customer_id);
CREATE INDEX prescriptions_status_idx          ON prescriptions(status);
CREATE INDEX prescriptions_expires_idx         ON prescriptions(expires_at);
CREATE INDEX prescription_items_rx_idx         ON prescription_items(prescription_id);
CREATE INDEX prescription_items_product_idx    ON prescription_items(product_id);

-- Sale links to a prescription when Rx-required products are dispensed. Nullable
-- FK with a NULL default, so the ADD COLUMN is allowed on an existing table.
ALTER TABLE sales ADD COLUMN prescription_id TEXT REFERENCES prescriptions(id);
CREATE INDEX sales_prescription_idx ON sales(prescription_id) WHERE prescription_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS sales_prescription_idx;
ALTER TABLE sales DROP COLUMN prescription_id;
DROP TABLE IF EXISTS prescription_items;
DROP TABLE IF EXISTS prescriptions;
DROP TABLE IF EXISTS rx_no_counters;

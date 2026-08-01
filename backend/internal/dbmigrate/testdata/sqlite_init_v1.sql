-- +goose Up
-- Consolidated SQLite schema, equivalent to the Postgres set at version 32.
-- Translation rules vs the postgres/ set:
--   uuid DEFAULT gen_random_uuid()  -> TEXT (primary keys filled app-side by the
--                                      db.registerUUIDDefault create-callback)
--   citext                          -> TEXT COLLATE NOCASE
--   timestamptz DEFAULT now()       -> DATETIME DEFAULT (datetime('now'))
--   bigint / boolean                -> INTEGER (booleans stored as 0/1)
--   foreign keys                    -> declared INLINE (SQLite can't ALTER ADD FK)
-- Mirror every future schema change into BOTH migration sets.

CREATE TABLE users (
    id            TEXT PRIMARY KEY NOT NULL,
    email         TEXT COLLATE NOCASE NOT NULL UNIQUE,
    name          TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('OWNER','PHARMACIST','CASHIER')),
    active        INTEGER NOT NULL DEFAULT 1,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE suppliers (
    id            TEXT PRIMARY KEY NOT NULL,
    name          TEXT NOT NULL,
    contact_email TEXT COLLATE NOCASE,
    phone         TEXT,
    active        INTEGER NOT NULL DEFAULT 1,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    code          TEXT NOT NULL
);

CREATE TABLE customers (
    id         TEXT PRIMARY KEY NOT NULL,
    name       TEXT NOT NULL,
    phone      TEXT NOT NULL DEFAULT '',
    notes      TEXT NOT NULL DEFAULT '',
    active     INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    address    TEXT NOT NULL DEFAULT ''
);

CREATE TABLE warehouses (
    id         TEXT PRIMARY KEY NOT NULL,
    code       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    address    TEXT NOT NULL DEFAULT '',
    phone      TEXT NOT NULL DEFAULT '',
    is_default INTEGER NOT NULL DEFAULT 0,
    active     INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE branches (
    id         TEXT PRIMARY KEY NOT NULL,
    code       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    address    TEXT NOT NULL DEFAULT '',
    phone      TEXT NOT NULL DEFAULT '',
    active     INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE products (
    id                    TEXT PRIMARY KEY NOT NULL,
    sku                   TEXT NOT NULL UNIQUE,
    name                  TEXT NOT NULL,
    unit                  TEXT NOT NULL,
    unit_price            INTEGER NOT NULL,
    prescription_required INTEGER NOT NULL DEFAULT 0,
    active                INTEGER NOT NULL DEFAULT 1,
    created_at            DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at            DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE product_units (
    id          TEXT PRIMARY KEY NOT NULL,
    product_id  TEXT NOT NULL REFERENCES products(id),
    name        TEXT NOT NULL,
    factor      INTEGER NOT NULL CHECK (factor > 0),
    is_base     INTEGER NOT NULL DEFAULT 0,
    sell_price  INTEGER NOT NULL DEFAULT 0,
    sellable    INTEGER NOT NULL DEFAULT 1,
    purchasable INTEGER NOT NULL DEFAULT 1,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    active      INTEGER NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE product_prices (
    id             TEXT PRIMARY KEY NOT NULL,
    product_id     TEXT NOT NULL REFERENCES products(id),
    unit_price     INTEGER NOT NULL,
    effective_from DATETIME NOT NULL DEFAULT (datetime('now')),
    effective_to   DATETIME,
    changed_by     TEXT NOT NULL REFERENCES users(id),
    created_at     DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE product_unit_prices (
    id              TEXT PRIMARY KEY NOT NULL,
    product_unit_id TEXT NOT NULL REFERENCES product_units(id),
    unit_sell_price INTEGER NOT NULL,
    effective_from  DATETIME NOT NULL DEFAULT (datetime('now')),
    effective_to    DATETIME,
    changed_by      TEXT REFERENCES users(id),
    created_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE batches (
    id           TEXT PRIMARY KEY NOT NULL,
    product_id   TEXT NOT NULL REFERENCES products(id),
    supplier_id  TEXT REFERENCES suppliers(id),
    batch_number TEXT NOT NULL DEFAULT '',
    expiry_date  DATE NOT NULL,
    cost_price   INTEGER NOT NULL DEFAULT 0,
    received_at  DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE sales (
    id              TEXT PRIMARY KEY NOT NULL,
    sale_no         TEXT UNIQUE,
    customer_id     TEXT REFERENCES customers(id),
    cashier_user_id TEXT NOT NULL REFERENCES users(id),
    payment_source  TEXT,
    subtotal        INTEGER NOT NULL DEFAULT 0,
    cart_discount   INTEGER NOT NULL DEFAULT 0,
    total           INTEGER NOT NULL DEFAULT 0,
    paid_amount     INTEGER NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','COMPLETED','VOIDED')),
    branch_id       TEXT,
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    completed_at    DATETIME,
    warehouse_id    TEXT REFERENCES warehouses(id)
);

CREATE TABLE sale_items (
    id                  TEXT PRIMARY KEY NOT NULL,
    sale_id             TEXT NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    product_id          TEXT NOT NULL REFERENCES products(id),
    batch_id            TEXT REFERENCES batches(id),
    qty                 INTEGER NOT NULL CHECK (qty > 0),
    unit_price_snapshot INTEGER NOT NULL DEFAULT 0,
    line_discount       INTEGER NOT NULL DEFAULT 0,
    line_total          INTEGER NOT NULL DEFAULT 0,
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    branch_id           TEXT,
    product_unit_id     TEXT,
    unit_name           TEXT NOT NULL DEFAULT '',
    unit_factor         INTEGER NOT NULL DEFAULT 1,
    base_qty            INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE stock_transfers (
    id                TEXT PRIMARY KEY NOT NULL,
    transfer_no       TEXT UNIQUE,
    from_warehouse_id TEXT NOT NULL REFERENCES warehouses(id),
    to_warehouse_id   TEXT NOT NULL REFERENCES warehouses(id),
    note              TEXT NOT NULL DEFAULT '',
    created_by        TEXT NOT NULL REFERENCES users(id),
    created_at        DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE stocktake_sessions (
    id           TEXT PRIMARY KEY NOT NULL,
    name         TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','COMPLETED','VOIDED')),
    branch_id    TEXT,
    created_by   TEXT NOT NULL REFERENCES users(id),
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    completed_at DATETIME,
    voided_at    DATETIME,
    warehouse_id TEXT REFERENCES warehouses(id)
);

CREATE TABLE stocktake_lines (
    id               TEXT PRIMARY KEY NOT NULL,
    session_id       TEXT NOT NULL REFERENCES stocktake_sessions(id) ON DELETE CASCADE,
    batch_id         TEXT NOT NULL REFERENCES batches(id),
    expected_qty     INTEGER NOT NULL,
    counted_qty      INTEGER,
    disposition      TEXT NOT NULL DEFAULT 'ADJUSTMENT' CHECK (disposition IN ('ADJUSTMENT','WRITE_OFF')),
    write_off_kind   TEXT CHECK (write_off_kind IS NULL OR write_off_kind IN ('EXPIRED','DAMAGED','LOST','THEFT','OTHER')),
    disposition_note TEXT NOT NULL DEFAULT '',
    counted_at       DATETIME,
    counted_by       TEXT REFERENCES users(id),
    created_at       DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (session_id, batch_id)
);

CREATE TABLE stock_movements (
    id                TEXT PRIMARY KEY NOT NULL,
    batch_id          TEXT NOT NULL REFERENCES batches(id),
    qty               INTEGER NOT NULL CHECK (qty <> 0),
    type              TEXT NOT NULL CHECK (type IN ('PURCHASE','SALE','ADJUSTMENT','WRITE_OFF','TRANSFER_IN','TRANSFER_OUT')),
    reason            TEXT NOT NULL DEFAULT '',
    user_id           TEXT NOT NULL REFERENCES users(id),
    created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    sale_item_id      TEXT REFERENCES sale_items(id),
    branch_id         TEXT,
    stocktake_line_id TEXT REFERENCES stocktake_lines(id),
    write_off_kind    TEXT CHECK (write_off_kind IS NULL OR write_off_kind IN ('EXPIRED','DAMAGED','LOST','THEFT','OTHER')),
    warehouse_id      TEXT NOT NULL REFERENCES warehouses(id),
    transfer_id       TEXT REFERENCES stock_transfers(id)
);

CREATE TABLE purchase_orders (
    id            TEXT PRIMARY KEY NOT NULL,
    po_no         TEXT UNIQUE,
    supplier_id   TEXT NOT NULL REFERENCES suppliers(id),
    status        TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','SENT','PARTIALLY_RECEIVED','RECEIVED','CLOSED','VOIDED')),
    invoice_date  DATE,
    note          TEXT NOT NULL DEFAULT '',
    ordered_total INTEGER NOT NULL DEFAULT 0,
    paid_amount   INTEGER NOT NULL DEFAULT 0,
    created_by    TEXT NOT NULL REFERENCES users(id),
    branch_id     TEXT,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    sent_at       DATETIME,
    closed_at     DATETIME,
    warehouse_id  TEXT NOT NULL REFERENCES warehouses(id),
    invoice_no    TEXT NOT NULL DEFAULT '',
    due_at        DATE,
    subtotal      INTEGER NOT NULL DEFAULT 0,
    cart_discount INTEGER NOT NULL DEFAULT 0,
    ppn_enabled   INTEGER NOT NULL DEFAULT 0,
    ppn_amount    INTEGER NOT NULL DEFAULT 0,
    ppn_rate      INTEGER NOT NULL DEFAULT 11
);

CREATE TABLE purchase_order_items (
    id                TEXT PRIMARY KEY NOT NULL,
    purchase_order_id TEXT NOT NULL REFERENCES purchase_orders(id) ON DELETE CASCADE,
    product_id        TEXT NOT NULL REFERENCES products(id),
    ordered_qty       INTEGER NOT NULL CHECK (ordered_qty > 0),
    received_qty      INTEGER NOT NULL DEFAULT 0,
    unit_cost_price   INTEGER NOT NULL DEFAULT 0,
    subtotal          INTEGER NOT NULL DEFAULT 0,
    product_unit_id   TEXT,
    unit_name         TEXT NOT NULL DEFAULT '',
    unit_factor       INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE purchase_receipts (
    id                TEXT PRIMARY KEY NOT NULL,
    receipt_no        TEXT UNIQUE,
    purchase_order_id TEXT NOT NULL REFERENCES purchase_orders(id),
    received_at       DATE NOT NULL DEFAULT CURRENT_DATE,
    received_by       TEXT NOT NULL REFERENCES users(id),
    note              TEXT NOT NULL DEFAULT '',
    created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    invoice_no        TEXT NOT NULL DEFAULT ''
);

CREATE TABLE purchase_receipt_items (
    id                     TEXT PRIMARY KEY NOT NULL,
    purchase_receipt_id    TEXT NOT NULL REFERENCES purchase_receipts(id) ON DELETE CASCADE,
    purchase_order_item_id TEXT NOT NULL REFERENCES purchase_order_items(id),
    product_id             TEXT NOT NULL REFERENCES products(id),
    qty                    INTEGER NOT NULL CHECK (qty > 0),
    unit_cost_price        INTEGER NOT NULL DEFAULT 0,
    batch_number           TEXT NOT NULL DEFAULT '',
    expiry_date            DATE NOT NULL,
    batch_id               TEXT REFERENCES batches(id),
    created_at             DATETIME NOT NULL DEFAULT (datetime('now')),
    product_unit_id        TEXT,
    unit_name              TEXT NOT NULL DEFAULT '',
    unit_factor            INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE refresh_tokens (
    id          TEXT PRIMARY KEY NOT NULL,
    user_id     TEXT NOT NULL REFERENCES users(id),
    token_hash  TEXT NOT NULL UNIQUE,
    family_id   TEXT NOT NULL,
    parent_id   TEXT REFERENCES refresh_tokens(id),
    expires_at  DATETIME NOT NULL,
    revoked_at  DATETIME,
    user_agent  TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE password_reset_tokens (
    id         TEXT PRIMARY KEY NOT NULL,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    issued_by  TEXT NOT NULL REFERENCES users(id),
    expires_at DATETIME NOT NULL,
    used_at    DATETIME,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE user_branches (
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    branch_id  TEXT NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    is_default INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (user_id, branch_id)
);

CREATE TABLE user_warehouses (
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    warehouse_id TEXT NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
    is_default   INTEGER NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (user_id, warehouse_id)
);

CREATE TABLE audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     TEXT REFERENCES users(id),
    role        TEXT NOT NULL DEFAULT '',
    branch_id   TEXT,
    procedure   TEXT NOT NULL,
    ok          INTEGER NOT NULL,
    code        TEXT NOT NULL DEFAULT '',
    message     TEXT NOT NULL DEFAULT '',
    ip          TEXT NOT NULL DEFAULT '',
    user_agent  TEXT NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE app_settings (
    key        TEXT PRIMARY KEY NOT NULL,
    value      TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE unit_bases (
    id         TEXT PRIMARY KEY NOT NULL,
    name       TEXT NOT NULL UNIQUE,
    active     INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE unit_derivatives (
    id           TEXT PRIMARY KEY NOT NULL,
    base_unit_id TEXT NOT NULL REFERENCES unit_bases(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    factor       INTEGER NOT NULL CHECK (factor > 1),
    sort_order   INTEGER NOT NULL DEFAULT 0,
    active       INTEGER NOT NULL DEFAULT 1,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE po_no_counters       (year INTEGER PRIMARY KEY NOT NULL, last_seq INTEGER NOT NULL DEFAULT 0);
CREATE TABLE rcv_no_counters      (year INTEGER PRIMARY KEY NOT NULL, last_seq INTEGER NOT NULL DEFAULT 0);
CREATE TABLE sale_no_counters     (year INTEGER PRIMARY KEY NOT NULL, last_seq INTEGER NOT NULL DEFAULT 0);
CREATE TABLE transfer_no_counters (year INTEGER PRIMARY KEY NOT NULL, last_seq INTEGER NOT NULL DEFAULT 0);

-- Indexes (partial WHERE clauses preserved; booleans compared against 1).
CREATE INDEX audit_log_created_idx        ON audit_log(created_at DESC);
CREATE INDEX audit_log_procedure_idx      ON audit_log(procedure, created_at DESC);
CREATE INDEX audit_log_user_idx           ON audit_log(user_id, created_at DESC);
CREATE INDEX batches_expiry_idx           ON batches(expiry_date);
CREATE INDEX batches_product_idx          ON batches(product_id);
CREATE INDEX customers_name_idx           ON customers(name);
CREATE INDEX customers_phone_idx          ON customers(phone) WHERE phone <> '';
CREATE INDEX password_reset_unused_idx    ON password_reset_tokens(expires_at) WHERE used_at IS NULL;
CREATE INDEX password_reset_user_idx      ON password_reset_tokens(user_id);
CREATE UNIQUE INDEX product_prices_open_idx       ON product_prices(product_id) WHERE effective_to IS NULL;
CREATE INDEX product_prices_product_idx           ON product_prices(product_id);
CREATE UNIQUE INDEX product_unit_prices_open_idx  ON product_unit_prices(product_unit_id) WHERE effective_to IS NULL;
CREATE INDEX product_unit_prices_unit_idx         ON product_unit_prices(product_unit_id);
CREATE UNIQUE INDEX product_units_base_idx        ON product_units(product_id) WHERE is_base = 1;
CREATE UNIQUE INDEX product_units_name_idx        ON product_units(product_id, name) WHERE active = 1;
CREATE INDEX product_units_product_idx            ON product_units(product_id);
CREATE INDEX products_name_idx            ON products(name);
CREATE INDEX purchase_order_items_po_idx          ON purchase_order_items(purchase_order_id);
CREATE INDEX purchase_order_items_product_idx     ON purchase_order_items(product_id);
CREATE INDEX purchase_orders_created_idx          ON purchase_orders(created_at);
CREATE INDEX purchase_orders_status_idx           ON purchase_orders(status);
CREATE INDEX purchase_orders_supplier_idx         ON purchase_orders(supplier_id);
CREATE INDEX purchase_orders_warehouse_idx        ON purchase_orders(warehouse_id, created_at DESC);
CREATE INDEX purchase_receipt_items_batch_idx     ON purchase_receipt_items(batch_id) WHERE batch_id IS NOT NULL;
CREATE INDEX purchase_receipt_items_poi_idx       ON purchase_receipt_items(purchase_order_item_id);
CREATE INDEX purchase_receipt_items_receipt_idx   ON purchase_receipt_items(purchase_receipt_id);
CREATE INDEX purchase_receipts_po_idx             ON purchase_receipts(purchase_order_id);
CREATE INDEX refresh_tokens_expires_idx   ON refresh_tokens(expires_at);
CREATE INDEX refresh_tokens_family_idx    ON refresh_tokens(family_id);
CREATE INDEX refresh_tokens_user_idx      ON refresh_tokens(user_id);
CREATE INDEX sale_items_product_idx       ON sale_items(product_id);
CREATE INDEX sale_items_sale_idx          ON sale_items(sale_id);
CREATE INDEX sales_completed_idx          ON sales(completed_at) WHERE completed_at IS NOT NULL;
CREATE INDEX sales_created_idx            ON sales(created_at);
CREATE INDEX sales_customer_idx           ON sales(customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX sales_status_idx             ON sales(status);
CREATE INDEX sales_warehouse_idx          ON sales(warehouse_id);
CREATE INDEX stock_movements_batch_idx       ON stock_movements(batch_id);
CREATE INDEX stock_movements_created_idx     ON stock_movements(created_at);
CREATE INDEX stock_movements_sale_item_idx   ON stock_movements(sale_item_id) WHERE sale_item_id IS NOT NULL;
CREATE INDEX stock_movements_stocktake_idx   ON stock_movements(stocktake_line_id) WHERE stocktake_line_id IS NOT NULL;
CREATE INDEX stock_movements_transfer_idx    ON stock_movements(transfer_id) WHERE transfer_id IS NOT NULL;
CREATE INDEX stock_movements_warehouse_idx   ON stock_movements(batch_id, warehouse_id);
CREATE INDEX stock_movements_writeoff_idx    ON stock_movements(write_off_kind) WHERE write_off_kind IS NOT NULL;
CREATE INDEX stock_transfers_created_idx  ON stock_transfers(created_at DESC);
CREATE INDEX stocktake_lines_batch_idx    ON stocktake_lines(batch_id);
CREATE INDEX stocktake_lines_session_idx  ON stocktake_lines(session_id);
CREATE INDEX stocktake_sessions_created_idx ON stocktake_sessions(created_at DESC);
CREATE INDEX stocktake_sessions_status_idx  ON stocktake_sessions(status);
CREATE UNIQUE INDEX suppliers_code_idx        ON suppliers(code);
CREATE UNIQUE INDEX suppliers_name_active_idx ON suppliers(name) WHERE active = 1;
CREATE UNIQUE INDEX unit_derivatives_unique_active ON unit_derivatives(base_unit_id, name) WHERE active = 1;
CREATE INDEX user_branches_branch_idx     ON user_branches(branch_id);
CREATE UNIQUE INDEX user_branches_default_idx    ON user_branches(user_id) WHERE is_default = 1;
CREATE UNIQUE INDEX user_warehouses_default_idx  ON user_warehouses(user_id) WHERE is_default = 1;
CREATE INDEX user_warehouses_warehouse_idx ON user_warehouses(warehouse_id);
CREATE INDEX users_role_idx               ON users(role);
CREATE UNIQUE INDEX warehouses_default_idx ON warehouses(is_default) WHERE is_default = 1;

-- Seed the single-shop defaults (matches the postgres set: a MAIN branch + a
-- default MAIN warehouse "Gudang Utama"). Fixed ids since SQLite has no
-- gen_random_uuid() and these are raw inserts (the Go callback only fires for
-- GORM inserts). The bootstrap owner's membership is granted in Go via
-- grantDefaultWarehouse, exactly as on Postgres.
INSERT INTO branches (id, code, name) VALUES
    ('00000000-0000-0000-0000-0000000000b1', 'MAIN', 'Main pharmacy');
INSERT INTO warehouses (id, code, name, is_default) VALUES
    ('00000000-0000-0000-0000-0000000000a1', 'MAIN', 'Gudang Utama', 1);

-- +goose Down
DROP TABLE IF EXISTS transfer_no_counters;
DROP TABLE IF EXISTS sale_no_counters;
DROP TABLE IF EXISTS rcv_no_counters;
DROP TABLE IF EXISTS po_no_counters;
DROP TABLE IF EXISTS unit_derivatives;
DROP TABLE IF EXISTS unit_bases;
DROP TABLE IF EXISTS app_settings;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS user_warehouses;
DROP TABLE IF EXISTS user_branches;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS purchase_receipt_items;
DROP TABLE IF EXISTS purchase_receipts;
DROP TABLE IF EXISTS purchase_order_items;
DROP TABLE IF EXISTS purchase_orders;
DROP TABLE IF EXISTS stock_movements;
DROP TABLE IF EXISTS stocktake_lines;
DROP TABLE IF EXISTS stocktake_sessions;
DROP TABLE IF EXISTS stock_transfers;
DROP TABLE IF EXISTS sale_items;
DROP TABLE IF EXISTS sales;
DROP TABLE IF EXISTS batches;
DROP TABLE IF EXISTS product_unit_prices;
DROP TABLE IF EXISTS product_prices;
DROP TABLE IF EXISTS product_units;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS branches;
DROP TABLE IF EXISTS warehouses;
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS suppliers;
DROP TABLE IF EXISTS users;

-- +goose NO TRANSACTION
-- Full-order refunds. SQLite can't ALTER a CHECK constraint, so rebuild the two
-- affected tables: sales (widen status CHECK to add 'REFUNDED' + add the 4
-- refund columns) and stock_movements (widen type CHECK to add 'RETURN').
-- Same table-rebuild pattern as 00035_user_role_apoteker.sql.

-- +goose Up
PRAGMA foreign_keys=OFF;

-- ---- sales: widen status CHECK + add refund columns ----
-- +goose StatementBegin
CREATE TABLE sales_new (
    id              TEXT PRIMARY KEY NOT NULL,
    sale_no         TEXT UNIQUE,
    customer_id     TEXT REFERENCES customers(id),
    cashier_user_id TEXT NOT NULL REFERENCES users(id),
    payment_source  TEXT,
    subtotal        INTEGER NOT NULL DEFAULT 0,
    cart_discount   INTEGER NOT NULL DEFAULT 0,
    total           INTEGER NOT NULL DEFAULT 0,
    paid_amount     INTEGER NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','COMPLETED','VOIDED','REFUNDED')),
    branch_id       TEXT,
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    completed_at    DATETIME,
    warehouse_id    TEXT REFERENCES warehouses(id),
    prescription_id TEXT REFERENCES prescriptions(id),
    biaya_jasa      INTEGER NOT NULL DEFAULT 0,
    cart_discount_type  TEXT    NOT NULL DEFAULT 'FIXED',
    cart_discount_value INTEGER NOT NULL DEFAULT 0,
    refunded_at      DATETIME,
    refund_amount    INTEGER NOT NULL DEFAULT 0,
    refund_reason    TEXT    NOT NULL DEFAULT '',
    refund_restocked INTEGER NOT NULL DEFAULT 0
);
-- +goose StatementEnd
INSERT INTO sales_new
  (id, sale_no, customer_id, cashier_user_id, payment_source, subtotal, cart_discount,
   total, paid_amount, status, branch_id, created_at, updated_at, completed_at,
   warehouse_id, prescription_id, biaya_jasa, cart_discount_type, cart_discount_value)
  SELECT id, sale_no, customer_id, cashier_user_id, payment_source, subtotal, cart_discount,
   total, paid_amount, status, branch_id, created_at, updated_at, completed_at,
   warehouse_id, prescription_id, biaya_jasa, cart_discount_type, cart_discount_value
  FROM sales;
DROP TABLE sales;
ALTER TABLE sales_new RENAME TO sales;
CREATE INDEX sales_completed_idx    ON sales(completed_at) WHERE completed_at IS NOT NULL;
CREATE INDEX sales_created_idx      ON sales(created_at);
CREATE INDEX sales_customer_idx     ON sales(customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX sales_status_idx       ON sales(status);
CREATE INDEX sales_warehouse_idx    ON sales(warehouse_id);
CREATE INDEX sales_prescription_idx ON sales(prescription_id) WHERE prescription_id IS NOT NULL;

-- ---- stock_movements: widen type CHECK to add 'RETURN' ----
-- +goose StatementBegin
CREATE TABLE stock_movements_new (
    id                TEXT PRIMARY KEY NOT NULL,
    batch_id          TEXT NOT NULL REFERENCES batches(id),
    qty               INTEGER NOT NULL CHECK (qty <> 0),
    type              TEXT NOT NULL CHECK (type IN ('PURCHASE','SALE','ADJUSTMENT','WRITE_OFF','TRANSFER_IN','TRANSFER_OUT','RETURN')),
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
-- +goose StatementEnd
INSERT INTO stock_movements_new
  (id, batch_id, qty, type, reason, user_id, created_at, sale_item_id, branch_id,
   stocktake_line_id, write_off_kind, warehouse_id, transfer_id)
  SELECT id, batch_id, qty, type, reason, user_id, created_at, sale_item_id, branch_id,
   stocktake_line_id, write_off_kind, warehouse_id, transfer_id
  FROM stock_movements;
DROP TABLE stock_movements;
ALTER TABLE stock_movements_new RENAME TO stock_movements;
CREATE INDEX stock_movements_batch_idx     ON stock_movements(batch_id);
CREATE INDEX stock_movements_created_idx   ON stock_movements(created_at);
CREATE INDEX stock_movements_sale_item_idx ON stock_movements(sale_item_id) WHERE sale_item_id IS NOT NULL;
CREATE INDEX stock_movements_stocktake_idx ON stock_movements(stocktake_line_id) WHERE stocktake_line_id IS NOT NULL;
CREATE INDEX stock_movements_transfer_idx  ON stock_movements(transfer_id) WHERE transfer_id IS NOT NULL;
CREATE INDEX stock_movements_warehouse_idx ON stock_movements(batch_id, warehouse_id);
CREATE INDEX stock_movements_writeoff_idx  ON stock_movements(write_off_kind) WHERE write_off_kind IS NOT NULL;

PRAGMA foreign_keys=ON;

-- +goose Down
PRAGMA foreign_keys=OFF;

-- ---- revert stock_movements (drop 'RETURN') ----
-- +goose StatementBegin
CREATE TABLE stock_movements_old (
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
-- +goose StatementEnd
INSERT INTO stock_movements_old
  (id, batch_id, qty, type, reason, user_id, created_at, sale_item_id, branch_id,
   stocktake_line_id, write_off_kind, warehouse_id, transfer_id)
  SELECT id, batch_id, qty, type, reason, user_id, created_at, sale_item_id, branch_id,
   stocktake_line_id, write_off_kind, warehouse_id, transfer_id
  FROM stock_movements WHERE type <> 'RETURN';
DROP TABLE stock_movements;
ALTER TABLE stock_movements_old RENAME TO stock_movements;
CREATE INDEX stock_movements_batch_idx     ON stock_movements(batch_id);
CREATE INDEX stock_movements_created_idx   ON stock_movements(created_at);
CREATE INDEX stock_movements_sale_item_idx ON stock_movements(sale_item_id) WHERE sale_item_id IS NOT NULL;
CREATE INDEX stock_movements_stocktake_idx ON stock_movements(stocktake_line_id) WHERE stocktake_line_id IS NOT NULL;
CREATE INDEX stock_movements_transfer_idx  ON stock_movements(transfer_id) WHERE transfer_id IS NOT NULL;
CREATE INDEX stock_movements_warehouse_idx ON stock_movements(batch_id, warehouse_id);
CREATE INDEX stock_movements_writeoff_idx  ON stock_movements(write_off_kind) WHERE write_off_kind IS NOT NULL;

-- ---- revert sales (drop refund columns + 'REFUNDED') ----
-- +goose StatementBegin
CREATE TABLE sales_old (
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
    warehouse_id    TEXT REFERENCES warehouses(id),
    prescription_id TEXT REFERENCES prescriptions(id),
    biaya_jasa      INTEGER NOT NULL DEFAULT 0,
    cart_discount_type  TEXT    NOT NULL DEFAULT 'FIXED',
    cart_discount_value INTEGER NOT NULL DEFAULT 0
);
-- +goose StatementEnd
INSERT INTO sales_old
  (id, sale_no, customer_id, cashier_user_id, payment_source, subtotal, cart_discount,
   total, paid_amount, status, branch_id, created_at, updated_at, completed_at,
   warehouse_id, prescription_id, biaya_jasa, cart_discount_type, cart_discount_value)
  SELECT id, sale_no, customer_id, cashier_user_id, payment_source, subtotal, cart_discount,
   total, paid_amount, CASE WHEN status = 'REFUNDED' THEN 'COMPLETED' ELSE status END, branch_id,
   created_at, updated_at, completed_at, warehouse_id, prescription_id, biaya_jasa,
   cart_discount_type, cart_discount_value
  FROM sales;
DROP TABLE sales;
ALTER TABLE sales_old RENAME TO sales;
CREATE INDEX sales_completed_idx    ON sales(completed_at) WHERE completed_at IS NOT NULL;
CREATE INDEX sales_created_idx      ON sales(created_at);
CREATE INDEX sales_customer_idx     ON sales(customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX sales_status_idx       ON sales(status);
CREATE INDEX sales_warehouse_idx    ON sales(warehouse_id);
CREATE INDEX sales_prescription_idx ON sales(prescription_id) WHERE prescription_id IS NOT NULL;

PRAGMA foreign_keys=ON;

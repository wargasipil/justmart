-- +goose NO TRANSACTION
-- Partial purchase returns ("retur pembelian"). SQLite can't ALTER a CHECK, so
-- rebuild stock_movements to widen the type CHECK with 'PURCHASE_RETURN'. The
-- returned_amount column on purchase_orders uses a plain ADD COLUMN (constant
-- default). Mirror of postgres 00043_purchase_returns.sql; same table-rebuild
-- pattern as 00041_refunds.sql.

-- +goose Up
PRAGMA foreign_keys=OFF;

-- Accumulated returned value per PO (drops outstanding).
ALTER TABLE purchase_orders ADD COLUMN returned_amount INTEGER NOT NULL DEFAULT 0;

-- ---- stock_movements: widen type CHECK to add 'PURCHASE_RETURN' ----
-- +goose StatementBegin
CREATE TABLE stock_movements_new (
    id                TEXT PRIMARY KEY NOT NULL,
    batch_id          TEXT NOT NULL REFERENCES batches(id),
    qty               INTEGER NOT NULL CHECK (qty <> 0),
    type              TEXT NOT NULL CHECK (type IN ('PURCHASE','SALE','ADJUSTMENT','WRITE_OFF','TRANSFER_IN','TRANSFER_OUT','RETURN','PURCHASE_RETURN')),
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

CREATE TABLE rtn_no_counters (
    year     INTEGER PRIMARY KEY,
    last_seq INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE purchase_returns (
    id                TEXT PRIMARY KEY NOT NULL,
    return_no         TEXT UNIQUE,
    purchase_order_id TEXT NOT NULL REFERENCES purchase_orders(id),
    warehouse_id      TEXT NOT NULL REFERENCES warehouses(id),
    returned_at       DATE NOT NULL DEFAULT CURRENT_DATE,
    returned_by       TEXT NOT NULL REFERENCES users(id),
    reason            TEXT    NOT NULL DEFAULT '',
    note              TEXT    NOT NULL DEFAULT '',
    refund_amount     INTEGER NOT NULL DEFAULT 0,
    created_at        DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX purchase_returns_po_idx ON purchase_returns(purchase_order_id);

CREATE TABLE purchase_return_items (
    id                       TEXT PRIMARY KEY NOT NULL,
    purchase_return_id       TEXT NOT NULL REFERENCES purchase_returns(id) ON DELETE CASCADE,
    purchase_receipt_item_id TEXT NOT NULL REFERENCES purchase_receipt_items(id),
    purchase_order_item_id   TEXT NOT NULL REFERENCES purchase_order_items(id),
    product_id               TEXT NOT NULL REFERENCES products(id),
    batch_id                 TEXT NOT NULL REFERENCES batches(id),
    qty                      INTEGER NOT NULL CHECK (qty > 0),
    unit_cost_price          INTEGER NOT NULL DEFAULT 0,
    unit_name                TEXT    NOT NULL DEFAULT '',
    unit_factor              INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX purchase_return_items_return_idx       ON purchase_return_items(purchase_return_id);
CREATE INDEX purchase_return_items_receipt_item_idx ON purchase_return_items(purchase_receipt_item_id);

PRAGMA foreign_keys=ON;

-- +goose Down
PRAGMA foreign_keys=OFF;
DROP TABLE IF EXISTS purchase_return_items;
DROP TABLE IF EXISTS purchase_returns;
DROP TABLE IF EXISTS rtn_no_counters;

-- ---- revert stock_movements (drop 'PURCHASE_RETURN' rows + narrow the CHECK) ----
-- +goose StatementBegin
CREATE TABLE stock_movements_old (
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
INSERT INTO stock_movements_old
  (id, batch_id, qty, type, reason, user_id, created_at, sale_item_id, branch_id,
   stocktake_line_id, write_off_kind, warehouse_id, transfer_id)
  SELECT id, batch_id, qty, type, reason, user_id, created_at, sale_item_id, branch_id,
   stocktake_line_id, write_off_kind, warehouse_id, transfer_id
  FROM stock_movements WHERE type <> 'PURCHASE_RETURN';
DROP TABLE stock_movements;
ALTER TABLE stock_movements_old RENAME TO stock_movements;
CREATE INDEX stock_movements_batch_idx     ON stock_movements(batch_id);
CREATE INDEX stock_movements_created_idx   ON stock_movements(created_at);
CREATE INDEX stock_movements_sale_item_idx ON stock_movements(sale_item_id) WHERE sale_item_id IS NOT NULL;
CREATE INDEX stock_movements_stocktake_idx ON stock_movements(stocktake_line_id) WHERE stocktake_line_id IS NOT NULL;
CREATE INDEX stock_movements_transfer_idx  ON stock_movements(transfer_id) WHERE transfer_id IS NOT NULL;
CREATE INDEX stock_movements_warehouse_idx ON stock_movements(batch_id, warehouse_id);
CREATE INDEX stock_movements_writeoff_idx  ON stock_movements(write_off_kind) WHERE write_off_kind IS NOT NULL;

PRAGMA foreign_keys=ON;

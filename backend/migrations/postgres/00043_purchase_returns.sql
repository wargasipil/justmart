-- +goose Up
-- Partial purchase returns ("retur pembelian"): send received goods back to the
-- supplier. Stock leaves the warehouse (negative PURCHASE_RETURN movement),
-- received_qty decrements (so a fully-received PO can reopen to PARTIALLY_RECEIVED),
-- and the supplier outstanding drops by the returned value (can go negative = a
-- supplier credit). Append-only ledger mirroring purchase_receipts.

-- New movement type: goods leaving to the supplier on a return (negative qty).
ALTER TABLE stock_movements DROP CONSTRAINT IF EXISTS stock_movements_type_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_type_check
  CHECK (type IN ('PURCHASE','SALE','ADJUSTMENT','WRITE_OFF','TRANSFER_IN','TRANSFER_OUT','RETURN','PURCHASE_RETURN'));

-- Accumulated returned value per PO (drops outstanding; can exceed what's still owed).
ALTER TABLE purchase_orders ADD COLUMN returned_amount BIGINT NOT NULL DEFAULT 0;

CREATE TABLE rtn_no_counters (
  year     INT PRIMARY KEY,
  last_seq INT NOT NULL DEFAULT 0
);

CREATE TABLE purchase_returns (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  return_no         TEXT UNIQUE,                       -- RTN-YYYY-NNNN
  purchase_order_id UUID NOT NULL REFERENCES purchase_orders(id),
  warehouse_id      UUID NOT NULL REFERENCES warehouses(id),
  returned_at       DATE NOT NULL DEFAULT CURRENT_DATE,
  returned_by       UUID NOT NULL REFERENCES users(id),
  reason            TEXT   NOT NULL DEFAULT '',
  note              TEXT   NOT NULL DEFAULT '',
  refund_amount     BIGINT NOT NULL DEFAULT 0,         -- sum of line (qty * unit_cost_price)
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX purchase_returns_po_idx ON purchase_returns(purchase_order_id);

CREATE TABLE purchase_return_items (
  id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  purchase_return_id       UUID NOT NULL REFERENCES purchase_returns(id) ON DELETE CASCADE,
  purchase_receipt_item_id UUID NOT NULL REFERENCES purchase_receipt_items(id),
  purchase_order_item_id   UUID NOT NULL REFERENCES purchase_order_items(id),
  product_id               UUID NOT NULL REFERENCES products(id),
  batch_id                 UUID NOT NULL REFERENCES batches(id),
  qty                      INTEGER NOT NULL CHECK (qty > 0),   -- BASE units returned
  unit_cost_price          BIGINT  NOT NULL DEFAULT 0,         -- per BASE unit (pinned from receipt item)
  unit_name                TEXT    NOT NULL DEFAULT '',
  unit_factor              BIGINT  NOT NULL DEFAULT 1
);
CREATE INDEX purchase_return_items_return_idx       ON purchase_return_items(purchase_return_id);
CREATE INDEX purchase_return_items_receipt_item_idx ON purchase_return_items(purchase_receipt_item_id);

-- +goose Down
DROP TABLE IF EXISTS purchase_return_items;
DROP TABLE IF EXISTS purchase_returns;
DROP TABLE IF EXISTS rtn_no_counters;
ALTER TABLE purchase_orders DROP COLUMN IF EXISTS returned_amount;
ALTER TABLE stock_movements DROP CONSTRAINT IF EXISTS stock_movements_type_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_type_check
  CHECK (type IN ('PURCHASE','SALE','ADJUSTMENT','WRITE_OFF','TRANSFER_IN','TRANSFER_OUT','RETURN'));

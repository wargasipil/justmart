-- +goose Up
-- Full-order refunds: a COMPLETED sale can be reversed to REFUNDED, returning
-- its goods to stock via RETURN movements. Widen the two CHECKs + add the
-- refund bookkeeping columns to sales.

-- New movement type for goods returned to stock on a refund (positive qty,
-- mirrors the sale's SALE movements).
ALTER TABLE stock_movements DROP CONSTRAINT IF EXISTS stock_movements_type_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_type_check
  CHECK (type IN ('PURCHASE','SALE','ADJUSTMENT','WRITE_OFF','TRANSFER_IN','TRANSFER_OUT','RETURN'));

-- New terminal sale status for a fully-refunded order.
ALTER TABLE sales DROP CONSTRAINT IF EXISTS sales_status_check;
ALTER TABLE sales ADD CONSTRAINT sales_status_check
  CHECK (status IN ('DRAFT','COMPLETED','VOIDED','REFUNDED'));

ALTER TABLE sales ADD COLUMN refunded_at      TIMESTAMPTZ;
ALTER TABLE sales ADD COLUMN refund_amount    BIGINT  NOT NULL DEFAULT 0;
ALTER TABLE sales ADD COLUMN refund_reason    TEXT    NOT NULL DEFAULT '';
ALTER TABLE sales ADD COLUMN refund_restocked BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE sales DROP COLUMN IF EXISTS refund_restocked;
ALTER TABLE sales DROP COLUMN IF EXISTS refund_reason;
ALTER TABLE sales DROP COLUMN IF EXISTS refund_amount;
ALTER TABLE sales DROP COLUMN IF EXISTS refunded_at;

ALTER TABLE sales DROP CONSTRAINT IF EXISTS sales_status_check;
ALTER TABLE sales ADD CONSTRAINT sales_status_check
  CHECK (status IN ('DRAFT','COMPLETED','VOIDED'));

ALTER TABLE stock_movements DROP CONSTRAINT IF EXISTS stock_movements_type_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_type_check
  CHECK (type IN ('PURCHASE','SALE','ADJUSTMENT','WRITE_OFF','TRANSFER_IN','TRANSFER_OUT'));

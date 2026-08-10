-- +goose Up
-- Cancel an accepted restock (receipt) that was entered in error.
--
-- Distinct from a purchase RETURN: a return means goods physically went back to
-- the supplier and accrues a refund/credit; a cancel means the receipt never
-- should have existed. It is only permitted while every lot the receipt created
-- is still UNTOUCHED (exactly one stock movement — the PURCHASE that created
-- it), so the lot and its movement are deleted outright and the PO reads as if
-- nothing ever arrived. The receipt DOCUMENT survives, marked voided: the
-- RCV-YYYY-NNNN number is already burned from the counter, and an unexplained
-- gap in that sequence is exactly what an audit questions.
--
-- purchase_receipt_items.batch_id is already nullable; cancelling nulls it
-- before the batch row is deleted.

ALTER TABLE purchase_receipts ADD COLUMN voided_at   TIMESTAMPTZ NULL;
ALTER TABLE purchase_receipts ADD COLUMN voided_by   UUID NULL REFERENCES users(id);
ALTER TABLE purchase_receipts ADD COLUMN void_reason TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE purchase_receipts DROP COLUMN IF EXISTS void_reason;
ALTER TABLE purchase_receipts DROP COLUMN IF EXISTS voided_by;
ALTER TABLE purchase_receipts DROP COLUMN IF EXISTS voided_at;

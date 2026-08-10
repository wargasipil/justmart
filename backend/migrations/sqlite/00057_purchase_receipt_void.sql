-- +goose Up
-- Mirror of postgres 00053_purchase_receipt_void.sql — see that file for why a
-- cancel deletes the lot instead of writing a compensating movement.
--
-- Plain ADD COLUMNs: no CHECK constraint changes, so no table rebuild is needed
-- here. SQLite permits ADD COLUMN with a REFERENCES clause as long as the
-- default is NULL, which it is.

ALTER TABLE purchase_receipts ADD COLUMN voided_at   DATETIME NULL;
ALTER TABLE purchase_receipts ADD COLUMN voided_by   TEXT NULL REFERENCES users(id);
ALTER TABLE purchase_receipts ADD COLUMN void_reason TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE purchase_receipts DROP COLUMN void_reason;
ALTER TABLE purchase_receipts DROP COLUMN voided_by;
ALTER TABLE purchase_receipts DROP COLUMN voided_at;

-- +goose Up
-- Mirror of postgres 00065_receipt_replaced_by.sql (SQLite dialect). See that
-- file for what the column means. A plain ADD COLUMN: nullable, no default, so
-- no table rebuild, and every existing receipt reads NULL = "not replaced".
ALTER TABLE purchase_receipts ADD COLUMN replaced_by_id TEXT NULL REFERENCES purchase_receipts(id);

-- +goose Down
ALTER TABLE purchase_receipts DROP COLUMN replaced_by_id;

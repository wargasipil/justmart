-- +goose Up
-- Per-item discount application for PO lines. When true, the line's discount_type
-- (FIXED minor units | PERCENT basis points) is applied to EACH item's cost and
-- summed across ordered_qty, instead of to the whole line. Additive boolean — no
-- discount_type CHECK change needed, so SQLite needs no table rebuild. Legacy rows
-- default to false => per-line behavior, net math unchanged.
ALTER TABLE purchase_order_items
  ADD COLUMN discount_per_item BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE purchase_order_items
  DROP COLUMN IF EXISTS discount_per_item;

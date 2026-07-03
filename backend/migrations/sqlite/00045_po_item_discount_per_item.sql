-- +goose Up
-- Per-item discount application for PO lines (mirror of postgres 00045). SQLite
-- has no BOOLEAN type and stores bools as INTEGER 0/1 (same as is_base /
-- ppn_enabled). Additive ADD COLUMN — no CHECK change, so no table rebuild.
ALTER TABLE purchase_order_items
  ADD COLUMN discount_per_item INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- Additive column is intentionally LEFT in place on down (same convention as
-- 00043's returned_amount) — dropping a column needs a table rebuild on older
-- SQLite, and an unused extra column is harmless. No-op.
SELECT 1;

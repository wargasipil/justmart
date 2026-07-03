-- +goose Up
-- Per-item discount flag into the restock-history tables (mirror of postgres
-- 00046). SQLite stores bools as INTEGER 0/1. Additive ADD COLUMN — no CHECK
-- change, so no table rebuild.
ALTER TABLE product_last_restocks
  ADD COLUMN last_discount_per_item INTEGER NOT NULL DEFAULT 0;
ALTER TABLE product_restock_logs
  ADD COLUMN discount_per_item INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- Additive columns intentionally LEFT in place on down (same convention as
-- 00045 / 00043) — dropping needs a rebuild on older SQLite. No-op.
SELECT 1;

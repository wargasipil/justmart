-- +goose Up
-- Carry the PO line's per-item discount flag into the restock-history tables so
-- the product-detail restock tab + supplier-detail restocks list can show a
-- per-item discount as such (e.g. "10% /item"). Additive booleans, no CHECK →
-- no table rebuild. Written by PurchaseReceiptService.CreateReceipt's recordRestock.
ALTER TABLE product_last_restocks
  ADD COLUMN last_discount_per_item BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE product_restock_logs
  ADD COLUMN discount_per_item BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE product_restock_logs
  DROP COLUMN IF EXISTS discount_per_item;
ALTER TABLE product_last_restocks
  DROP COLUMN IF EXISTS last_discount_per_item;

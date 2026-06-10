-- +goose Up
-- Per-line discount on PO items. FIXED = minor units (rupiah); PERCENT = basis
-- points (percent*100, so 12.5% = 1250). unit_cost_price stays GROSS per base
-- unit; subtotal is now the NET (post-line-discount) extended amount. Legacy
-- rows default to FIXED/0 => net == gross, so ordered_total math is unchanged.
ALTER TABLE purchase_order_items
  ADD COLUMN discount_type  TEXT   NOT NULL DEFAULT 'FIXED'
                            CHECK (discount_type IN ('FIXED','PERCENT')),
  ADD COLUMN discount_value BIGINT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE purchase_order_items
  DROP COLUMN IF EXISTS discount_value,
  DROP COLUMN IF EXISTS discount_type;

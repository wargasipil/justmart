-- +goose Up
-- POS discounts: per-line + cart-level discount, by fixed amount or percent.
-- The existing line_discount / cart_discount columns keep the RESOLVED amount
-- (minor units, kept in sync on every recompute) so analytics/order-detail are
-- unchanged; these new columns persist the discount "info": type + raw value.
-- discount_value is minor units when FIXED, basis points (percent*100) when PERCENT.
ALTER TABLE sale_items ADD COLUMN discount_type      TEXT   NOT NULL DEFAULT 'FIXED';
ALTER TABLE sale_items ADD COLUMN discount_value     BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sales      ADD COLUMN cart_discount_type  TEXT   NOT NULL DEFAULT 'FIXED';
ALTER TABLE sales      ADD COLUMN cart_discount_value BIGINT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE sales      DROP COLUMN IF EXISTS cart_discount_value;
ALTER TABLE sales      DROP COLUMN IF EXISTS cart_discount_type;
ALTER TABLE sale_items DROP COLUMN IF EXISTS discount_value;
ALTER TABLE sale_items DROP COLUMN IF EXISTS discount_type;

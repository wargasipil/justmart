-- +goose Up
-- Discounts: owner-defined discounts, managed via the Discount menu (NOT yet
-- applied at POS — that's a follow-up). Two modes:
--   CODE    — a unique coupon code → FIXED/PERCENT value.
--   PRODUCT — targets a set of products (discount_products) + a buying threshold
--             (min_qty = minimum quantity) → FIXED/PERCENT value.
-- discount_value is minor units when FIXED, basis points (percent*100) when
-- PERCENT (same convention as sale_items.discount_value). expires_at optional.
CREATE TABLE discounts (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name           TEXT NOT NULL,
  mode           TEXT NOT NULL CHECK (mode IN ('CODE', 'PRODUCT')),
  code           TEXT NOT NULL DEFAULT '',
  discount_type  TEXT NOT NULL DEFAULT 'PERCENT' CHECK (discount_type IN ('FIXED', 'PERCENT')),
  discount_value BIGINT NOT NULL DEFAULT 0,   -- FIXED=minor units; PERCENT=basis points
  min_qty        INTEGER NOT NULL DEFAULT 0,  -- buying threshold (PRODUCT mode); 0 = none
  expires_at     DATE,                        -- optional expiry
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- One discount per code (CODE mode); blank codes (PRODUCT mode) are exempt.
CREATE UNIQUE INDEX discounts_code_key ON discounts(code) WHERE code <> '';

CREATE TABLE discount_products (
  discount_id UUID NOT NULL REFERENCES discounts(id) ON DELETE CASCADE,
  product_id  UUID NOT NULL REFERENCES products(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (discount_id, product_id)
);
CREATE INDEX discount_products_product_idx ON discount_products(product_id);

-- +goose Down
DROP TABLE IF EXISTS discount_products;
DROP TABLE IF EXISTS discounts;

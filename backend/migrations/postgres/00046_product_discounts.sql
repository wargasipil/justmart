-- +goose Up
-- Per-product discounts (defined in the Product detail "Discount" tab, auto-applied
-- at POS). Replaces the standalone discounts/discount_products tables (00044), which
-- are dropped here. A product can have several discounts; each is one of 4 modes =
-- (discount_type FIXED|PERCENT) × per_item, gated by an optional min_qty rule and an
-- optional expiry. value = minor units when FIXED, basis points (percent*100) when
-- PERCENT. Mirrors the purchase-order line-discount model (per_item).
DROP TABLE IF EXISTS discount_products;
DROP TABLE IF EXISTS discounts;

CREATE TABLE product_discounts (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id    UUID NOT NULL REFERENCES products(id),
  discount_type TEXT NOT NULL DEFAULT 'PERCENT' CHECK (discount_type IN ('FIXED', 'PERCENT')),
  per_item      BOOLEAN NOT NULL DEFAULT false,
  value         BIGINT NOT NULL DEFAULT 0,   -- FIXED=minor units; PERCENT=basis points
  min_qty       INTEGER NOT NULL DEFAULT 0,  -- rule: minimal items to buy (per line, selling unit); 0/1 = always
  expires_at    DATE,                        -- optional expiry
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX product_discounts_product_idx ON product_discounts(product_id);

-- POS line carries the auto-discount's per-item flag (for rescale + display) and a
-- manual-override flag (cashier set the line discount, so don't re-resolve auto).
ALTER TABLE sale_items ADD COLUMN discount_per_item BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE sale_items ADD COLUMN discount_manual   BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE sale_items DROP COLUMN IF EXISTS discount_manual;
ALTER TABLE sale_items DROP COLUMN IF EXISTS discount_per_item;
DROP TABLE IF EXISTS product_discounts;
-- Restore the standalone discount tables (reverse of 00044).
CREATE TABLE discounts (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name           TEXT NOT NULL,
  mode           TEXT NOT NULL CHECK (mode IN ('CODE', 'PRODUCT')),
  code           TEXT NOT NULL DEFAULT '',
  discount_type  TEXT NOT NULL DEFAULT 'PERCENT' CHECK (discount_type IN ('FIXED', 'PERCENT')),
  discount_value BIGINT NOT NULL DEFAULT 0,
  min_qty        INTEGER NOT NULL DEFAULT 0,
  expires_at     DATE,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX discounts_code_key ON discounts(code) WHERE code <> '';
CREATE TABLE discount_products (
  discount_id UUID NOT NULL REFERENCES discounts(id) ON DELETE CASCADE,
  product_id  UUID NOT NULL REFERENCES products(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (discount_id, product_id)
);
CREATE INDEX discount_products_product_idx ON discount_products(product_id);

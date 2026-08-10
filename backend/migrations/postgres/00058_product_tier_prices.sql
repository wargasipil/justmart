-- +goose Up
-- Grosir (wholesale) tier price history, mirroring product_unit_prices but keyed
-- by the RUNG rather than the tier row.
--
-- product_price_tiers rows are HARD-deleted (DeleteProductPriceTier), so a
-- tier_id FK would dangle the moment a shop tidies its ladder. The thing that
-- actually has a price over time is the rung — ">= min_qty of THIS unit" — so
-- that pair is the key, and the history stays continuous across a
-- delete-then-recreate of the same rung. It also makes an edit that MOVES a tier
-- (min_qty 12 -> 24) read correctly: the >=12 rung closes and the >=24 rung
-- opens, which is what happened.
--
-- Exactly one open row (effective_to IS NULL) per rung. changed_by is nullable so
-- the backfill below (system-seeded baseline, no user) can insert without one.
CREATE TABLE product_tier_prices (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id      UUID NOT NULL REFERENCES products(id),
  product_unit_id UUID NOT NULL REFERENCES product_units(id),
  unit_name       TEXT    NOT NULL DEFAULT '',
  min_qty         INTEGER NOT NULL CHECK (min_qty >= 2),
  price           BIGINT  NOT NULL CHECK (price >= 0),
  effective_from  TIMESTAMPTZ NOT NULL DEFAULT now(),
  effective_to    TIMESTAMPTZ,
  changed_by      UUID REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX product_tier_prices_open_idx
  ON product_tier_prices(product_unit_id, min_qty)
  WHERE effective_to IS NULL;
CREATE INDEX product_tier_prices_product_idx ON product_tier_prices(product_id);

-- Backfill: one open row per existing tier at its current price. effective_from
-- is updated_at, NOT created_at — an edited tier's CURRENT price took effect at
-- its last edit, and the pre-history prices are unrecoverable either way.
INSERT INTO product_tier_prices (product_id, product_unit_id, unit_name, min_qty, price, effective_from)
SELECT product_id, product_unit_id, unit_name, min_qty, price, updated_at
FROM product_price_tiers;

-- +goose Down
DROP TABLE IF EXISTS product_tier_prices;

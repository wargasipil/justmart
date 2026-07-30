-- +goose Up
-- Grosir (wholesale) quantity price tiers. A product unit can have several tiers
-- forming a ladder; each says "buy >= min_qty of THIS unit and the whole line
-- re-prices to `price` per unit" (minor units). The tier REPLACES the unit's
-- sell_price on the line rather than discounting it, so gross = qty x price stays
-- exact; POS also suppresses the automatic product_discount on a tiered line (a
-- manual cashier discount still applies).
--
-- min_qty >= 2: a 0/1 tier would just be the unit's sell_price, and since
-- sale_items.tier_min_qty = 0 is the "no grosir" flag, allowing 0 would let one
-- bad row silently disable every auto discount for the product.
-- unit_name/unit_factor are display snapshots (mirrors price_agreements); the POS
-- gate compares sale_items.qty >= min_qty, counted in the tier's own unit.
CREATE TABLE product_price_tiers (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id      UUID NOT NULL REFERENCES products(id),
  product_unit_id UUID NOT NULL REFERENCES product_units(id),
  unit_name       TEXT    NOT NULL DEFAULT '',
  unit_factor     BIGINT  NOT NULL DEFAULT 1,
  min_qty         INTEGER NOT NULL CHECK (min_qty >= 2),
  price           BIGINT  NOT NULL DEFAULT 0 CHECK (price >= 0),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX product_price_tiers_unit_min_qty_key ON product_price_tiers(product_unit_id, min_qty);
CREATE INDEX product_price_tiers_product_idx ON product_price_tiers(product_id);

-- The POS line carries the unit's NORMAL catalog price frozen at add time, so
-- tier resolution is a pure function of (list_price_snapshot, qty): dropping the
-- qty back below a threshold restores the normal price instead of ratcheting.
-- It also gives the UI the strike-through value. tier_min_qty = the applied
-- tier's threshold; 0 = no grosir on this line (doubles as the "applied" flag).
ALTER TABLE sale_items ADD COLUMN list_price_snapshot BIGINT  NOT NULL DEFAULT 0;
ALTER TABLE sale_items ADD COLUMN tier_min_qty        INTEGER NOT NULL DEFAULT 0;
-- Backfill: before grosir, unit_price_snapshot WAS the catalog price by
-- definition (every write of it was `= unit.sell_price`).
UPDATE sale_items SET list_price_snapshot = unit_price_snapshot;

-- +goose Down
ALTER TABLE sale_items DROP COLUMN IF EXISTS tier_min_qty;
ALTER TABLE sale_items DROP COLUMN IF EXISTS list_price_snapshot;
DROP TABLE IF EXISTS product_price_tiers;

-- +goose Up
-- Supplier price agreements: the negotiated/agreed purchase price for a product
-- from a supplier, quoted per a chosen purchasable unit (box/strip/pcs). These
-- are reference records (distinct from product_last_restocks, which are ACTUALS).
-- Company-wide (no warehouse). unit_name/unit_factor are snapshotted from the
-- product_unit at write time (mirrors purchase_order_items). One active
-- agreement per (supplier, product, unit) — the partial unique index enforces it.
CREATE TABLE price_agreements (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  supplier_id     UUID NOT NULL REFERENCES suppliers(id),
  product_id      UUID NOT NULL REFERENCES products(id),
  product_unit_id UUID NOT NULL REFERENCES product_units(id),
  unit_name       TEXT   NOT NULL DEFAULT '',
  unit_factor     BIGINT NOT NULL DEFAULT 1,
  price           BIGINT NOT NULL DEFAULT 0,   -- agreed cost for 1 of product_unit (minor units)
  valid_from      DATE,
  valid_until     DATE,
  note            TEXT   NOT NULL DEFAULT '',
  active          BOOLEAN NOT NULL DEFAULT true,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX price_agreements_active_key ON price_agreements(supplier_id, product_id, product_unit_id) WHERE active;
CREATE INDEX price_agreements_supplier_idx ON price_agreements(supplier_id) WHERE active;
CREATE INDEX price_agreements_product_idx  ON price_agreements(product_id) WHERE active;

-- +goose Down
DROP TABLE IF EXISTS price_agreements;

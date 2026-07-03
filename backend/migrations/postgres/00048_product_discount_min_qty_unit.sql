-- +goose Up
-- Product-discount threshold gains a UNIT: the rule becomes "buy >= min_qty of
-- <unit>" (e.g. 3 box). min_qty_unit_id references the product's unit; the
-- name/factor are snapshotted at write time (mirrors price_agreements /
-- sale_items). NULL unit = the base unit (factor 1). The POS gate compares in
-- BASE units: base_qty >= min_qty * min_qty_unit_factor.
ALTER TABLE product_discounts ADD COLUMN min_qty_unit_id UUID REFERENCES product_units(id);
ALTER TABLE product_discounts ADD COLUMN min_qty_unit_name TEXT NOT NULL DEFAULT '';
ALTER TABLE product_discounts ADD COLUMN min_qty_unit_factor BIGINT NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE product_discounts DROP COLUMN IF EXISTS min_qty_unit_factor;
ALTER TABLE product_discounts DROP COLUMN IF EXISTS min_qty_unit_name;
ALTER TABLE product_discounts DROP COLUMN IF EXISTS min_qty_unit_id;

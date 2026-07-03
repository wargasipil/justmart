-- +goose Up
-- Mirror of postgres 00047_product_discount_min_qty_unit.sql (SQLite dialect).
ALTER TABLE product_discounts ADD COLUMN min_qty_unit_id TEXT REFERENCES product_units(id);
ALTER TABLE product_discounts ADD COLUMN min_qty_unit_name TEXT NOT NULL DEFAULT '';
ALTER TABLE product_discounts ADD COLUMN min_qty_unit_factor INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE product_discounts DROP COLUMN min_qty_unit_factor;
ALTER TABLE product_discounts DROP COLUMN min_qty_unit_name;
ALTER TABLE product_discounts DROP COLUMN min_qty_unit_id;

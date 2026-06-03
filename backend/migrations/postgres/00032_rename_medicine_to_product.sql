-- +goose Up
-- Retail conversion (justmart): rename the Medicine domain to Product. Tables,
-- FK columns, and indexes are renamed in lockstep with the Go models
-- (internal/model/product*.go) and the regenerated proto types. Runs after the
-- pharmacy-domain drop (00031), so prescription_items (the other medicine_id
-- holder) is already gone.

-- Tables. FKs that target these tables follow the rename automatically.
ALTER TABLE medicines            RENAME TO products;
ALTER TABLE medicine_prices      RENAME TO product_prices;
ALTER TABLE medicine_units       RENAME TO product_units;
ALTER TABLE medicine_unit_prices RENAME TO product_unit_prices;

-- Catalog FK columns: medicine_id -> product_id.
ALTER TABLE product_prices         RENAME COLUMN medicine_id TO product_id;
ALTER TABLE product_units          RENAME COLUMN medicine_id TO product_id;
ALTER TABLE batches                RENAME COLUMN medicine_id TO product_id;
ALTER TABLE sale_items             RENAME COLUMN medicine_id TO product_id;
ALTER TABLE purchase_order_items   RENAME COLUMN medicine_id TO product_id;
ALTER TABLE purchase_receipt_items RENAME COLUMN medicine_id TO product_id;

-- Selling-unit FK columns: medicine_unit_id -> product_unit_id.
ALTER TABLE sale_items             RENAME COLUMN medicine_unit_id TO product_unit_id;
ALTER TABLE purchase_order_items   RENAME COLUMN medicine_unit_id TO product_unit_id;
ALTER TABLE purchase_receipt_items RENAME COLUMN medicine_unit_id TO product_unit_id;
ALTER TABLE product_unit_prices    RENAME COLUMN medicine_unit_id TO product_unit_id;

-- Indexes (Postgres keeps the old names across a table rename).
ALTER INDEX medicines_name_idx               RENAME TO products_name_idx;
ALTER INDEX medicine_prices_open_idx         RENAME TO product_prices_open_idx;
ALTER INDEX medicine_prices_medicine_idx     RENAME TO product_prices_product_idx;
ALTER INDEX batches_medicine_idx             RENAME TO batches_product_idx;
ALTER INDEX sale_items_medicine_idx          RENAME TO sale_items_product_idx;
ALTER INDEX purchase_order_items_medicine_idx RENAME TO purchase_order_items_product_idx;
ALTER INDEX medicine_units_name_idx          RENAME TO product_units_name_idx;
ALTER INDEX medicine_units_base_idx          RENAME TO product_units_base_idx;
ALTER INDEX medicine_units_medicine_idx      RENAME TO product_units_product_idx;
ALTER INDEX medicine_unit_prices_open_idx    RENAME TO product_unit_prices_open_idx;
ALTER INDEX medicine_unit_prices_unit_idx    RENAME TO product_unit_prices_unit_idx;

-- +goose Down
ALTER INDEX product_unit_prices_unit_idx     RENAME TO medicine_unit_prices_unit_idx;
ALTER INDEX product_unit_prices_open_idx     RENAME TO medicine_unit_prices_open_idx;
ALTER INDEX product_units_product_idx        RENAME TO medicine_units_medicine_idx;
ALTER INDEX product_units_base_idx           RENAME TO medicine_units_base_idx;
ALTER INDEX product_units_name_idx           RENAME TO medicine_units_name_idx;
ALTER INDEX purchase_order_items_product_idx RENAME TO purchase_order_items_medicine_idx;
ALTER INDEX sale_items_product_idx           RENAME TO sale_items_medicine_idx;
ALTER INDEX batches_product_idx              RENAME TO batches_medicine_idx;
ALTER INDEX product_prices_product_idx       RENAME TO medicine_prices_medicine_idx;
ALTER INDEX product_prices_open_idx          RENAME TO medicine_prices_open_idx;
ALTER INDEX products_name_idx                RENAME TO medicines_name_idx;

ALTER TABLE product_unit_prices    RENAME COLUMN product_unit_id TO medicine_unit_id;
ALTER TABLE purchase_receipt_items RENAME COLUMN product_unit_id TO medicine_unit_id;
ALTER TABLE purchase_order_items   RENAME COLUMN product_unit_id TO medicine_unit_id;
ALTER TABLE sale_items             RENAME COLUMN product_unit_id TO medicine_unit_id;

ALTER TABLE purchase_receipt_items RENAME COLUMN product_id TO medicine_id;
ALTER TABLE purchase_order_items   RENAME COLUMN product_id TO medicine_id;
ALTER TABLE sale_items             RENAME COLUMN product_id TO medicine_id;
ALTER TABLE batches                RENAME COLUMN product_id TO medicine_id;
ALTER TABLE product_units          RENAME COLUMN product_id TO medicine_id;
ALTER TABLE product_prices         RENAME COLUMN product_id TO medicine_id;

ALTER TABLE product_unit_prices RENAME TO medicine_unit_prices;
ALTER TABLE product_units       RENAME TO medicine_units;
ALTER TABLE product_prices      RENAME TO medicine_prices;
ALTER TABLE products            RENAME TO medicines;

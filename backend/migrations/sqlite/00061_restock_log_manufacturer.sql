-- +goose Up
-- Mirror of postgres 00061_restock_log_manufacturer.sql (SQLite dialect). See
-- that file for why the maker is snapshotted onto the log row rather than
-- joined back through the receipt, and why it stays NULL on history.
-- UUID->TEXT. No table rebuild: one plain nullable column, which SQLite's
-- ALTER TABLE ADD COLUMN accepts with a REFERENCES clause when it defaults NULL.
ALTER TABLE product_restock_logs ADD COLUMN manufacturer_id TEXT REFERENCES manufacturers(id);
CREATE INDEX product_restock_logs_manufacturer_idx
  ON product_restock_logs(manufacturer_id) WHERE manufacturer_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS product_restock_logs_manufacturer_idx;
ALTER TABLE product_restock_logs DROP COLUMN manufacturer_id;

-- +goose Up
-- The restock log records what a delivery cost and who sold it. Since 00060 a
-- delivery also records who MADE it -- read off the distributor's invoice onto
-- the purchase-order line, then stamped onto the lot at receive.
--
-- The log is the per-product history of exactly that event, and it already
-- snapshots the line's price, qty and discount rather than joining back to the
-- order (the order can be edited; the log is what happened). The maker belongs
-- there on the same footing: without it, "we bought this generic from PBF Kimia
-- Farma four times" cannot answer WHOSE tablets arrived on which of them.
--
-- NULL on every historical row, permanently -- the same reason batches carries
-- NULL. The fact was not captured at the time, and back-filling it from the
-- product's current maker would invent provenance. A blank is honest; a guess
-- is not, and only the blank is recognisable as not-recorded.
ALTER TABLE product_restock_logs ADD COLUMN manufacturer_id UUID REFERENCES manufacturers(id);
CREATE INDEX product_restock_logs_manufacturer_idx
  ON product_restock_logs(manufacturer_id) WHERE manufacturer_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS product_restock_logs_manufacturer_idx;
ALTER TABLE product_restock_logs DROP COLUMN IF EXISTS manufacturer_id;

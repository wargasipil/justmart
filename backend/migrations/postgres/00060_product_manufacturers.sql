-- +goose Up
-- "Which pabrik" turned out to be TWO facts sharing one column.
--
-- products.manufacturer_id could only say who USUALLY makes an item. A shop
-- buying the same generic from three pabrik had nowhere to put the other two
-- and -- worse -- nowhere to record which of them a given lot on the shelf
-- actually came from. Restocking from a second maker overwrote the column,
-- silently relabelling every earlier batch, because nothing else held the fact.
--
-- So the two facts get two homes: the APPROVED-SOURCE LIST on the product
-- ("may be made by"), and the FACT on the lot ("was made by"). A list of
-- approved sources answers a buyer's question; only the lot answers a recall.
--
-- products.manufacturer_id SURVIVES as the PRIMARY maker -- a denormalised
-- pointer into the set below, the same relationship products.unit_price has
-- with product_prices -- so every existing read keeps working untouched.
CREATE TABLE product_manufacturers (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id      UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  manufacturer_id UUID NOT NULL REFERENCES manufacturers(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX product_manufacturers_pair_idx
  ON product_manufacturers(product_id, manufacturer_id);
-- Reverse lookup: "everything this pabrik is approved for".
CREATE INDEX product_manufacturers_manufacturer_idx
  ON product_manufacturers(manufacturer_id);

-- Backfill, so no product loses its pabrik and the invariant the service
-- enforces from here on -- the primary is always a member of its own set --
-- is already true of every existing row.
INSERT INTO product_manufacturers (product_id, manufacturer_id)
SELECT id, manufacturer_id FROM products WHERE manufacturer_id IS NOT NULL;

-- Who made THIS lot. NULL on every historical batch, permanently: the fact was
-- never recorded, and deriving it from the product's current maker would
-- invent provenance for stock that predates the question being asked.
ALTER TABLE batches ADD COLUMN manufacturer_id UUID REFERENCES manufacturers(id);
CREATE INDEX batches_manufacturer_idx
  ON batches(manufacturer_id) WHERE manufacturer_id IS NOT NULL;

-- What the buyer ordered, which is what CreateReceipt stamps onto the lot.
-- Nullable: a line that names no pabrik yields a lot that names none either,
-- rather than one guessed from the catalog.
ALTER TABLE purchase_order_items ADD COLUMN manufacturer_id UUID REFERENCES manufacturers(id);
CREATE INDEX purchase_order_items_manufacturer_idx
  ON purchase_order_items(manufacturer_id) WHERE manufacturer_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS purchase_order_items_manufacturer_idx;
ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS manufacturer_id;
DROP INDEX IF EXISTS batches_manufacturer_idx;
ALTER TABLE batches DROP COLUMN IF EXISTS manufacturer_id;
DROP TABLE IF EXISTS product_manufacturers;

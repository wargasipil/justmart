-- +goose Up
-- Product kinds + recipes (bill of materials).
--
-- product_kind discriminates what selling one costs the inventory:
--   STOCKED   - the default and every pre-existing row: the product IS the stock,
--               so a sale FEFO-consumes its own batches (unchanged behaviour).
--   COMPOSITE - a menu item / bundle: it has no batches of its own. Selling it
--               explodes through product_recipe_items and consumes the COMPONENTS.
--   SERVICE   - no stock at all (corkage, delivery fee, a plating charge). Sells
--               and prices like a product but consumes nothing.
--
-- This is deliberately NOT gated on the restaurant business mode: a retail shop
-- selling a gift bundle, or repacking a 25 kg sack into pouches, needs the exact
-- same engine, and a mode-gated one would change how an existing cart consumes
-- stock the moment the owner flipped modes.
ALTER TABLE products ADD COLUMN product_kind TEXT NOT NULL DEFAULT 'STOCKED';
ALTER TABLE products ADD CONSTRAINT products_kind_check
  CHECK (product_kind IN ('STOCKED','COMPOSITE','SERVICE'));

-- One row per ingredient line of a COMPOSITE product's recipe.
--
-- qty_base is the component's BASE units consumed per ONE BASE unit of the
-- parent — the same "everything is base units" rule the stock ledger, FEFO and
-- Rx coverage already follow, so selling 2 portions consumes 2 x qty_base and no
-- caller has to reason about selling units.
--
-- The self-reference CHECK is the cheap half of cycle protection; the service
-- also requires every component to be STOCKED, which makes deeper cycles
-- unrepresentable (a component can never itself be COMPOSITE). Nested recipes
-- are a documented non-goal, not an oversight.
CREATE TABLE product_recipe_items (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id           UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  component_product_id UUID NOT NULL REFERENCES products(id),
  qty_base             BIGINT NOT NULL CHECK (qty_base > 0),
  note                 TEXT NOT NULL DEFAULT '',
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT product_recipe_items_no_self CHECK (product_id <> component_product_id)
);
CREATE UNIQUE INDEX product_recipe_items_product_component_key
  ON product_recipe_items(product_id, component_product_id);
-- Reverse lookup: "which menu items use this ingredient" (component detail page,
-- and the guard that refuses to archive an ingredient still on a live recipe).
CREATE INDEX product_recipe_items_component_idx ON product_recipe_items(component_product_id);

-- +goose Down
DROP TABLE IF EXISTS product_recipe_items;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_kind_check;
ALTER TABLE products DROP COLUMN IF EXISTS product_kind;

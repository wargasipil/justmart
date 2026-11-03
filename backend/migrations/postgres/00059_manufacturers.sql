-- +goose Up
-- Manufacturers ("pabrik") — who MADE the goods, as distinct from the supplier
-- ("pemasok") who SOLD them to this shop. Separate tables because the relation
-- is many-to-many through reality: one pabrik reaches a shop via several
-- distributors, one distributor carries many pabrik, and a shop that buys the
-- same product from three PBFs still has exactly one manufacturer for it.
--
-- Deliberately NO bank/rekening columns (suppliers have those): a shop pays the
-- distributor, never the factory.
CREATE TABLE manufacturers (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code          TEXT NOT NULL,
  name          TEXT NOT NULL,
  address       TEXT NOT NULL DEFAULT '',
  phone         TEXT NOT NULL DEFAULT '',
  contact_email TEXT NOT NULL DEFAULT '',
  note          TEXT NOT NULL DEFAULT '',
  active        BOOLEAN NOT NULL DEFAULT TRUE,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Same index shape as suppliers: the code is globally unique, the name only
-- among ACTIVE rows, so archiving frees a name for re-use.
CREATE UNIQUE INDEX manufacturers_code_idx ON manufacturers(code);
CREATE UNIQUE INDEX manufacturers_name_active_idx ON manufacturers(name) WHERE active = TRUE;

-- The product link. NULL = not recorded, which is every product until someone
-- sets it — that is why there is no default and no backfill.
ALTER TABLE products ADD COLUMN manufacturer_id UUID REFERENCES manufacturers(id);
CREATE INDEX products_manufacturer_idx ON products(manufacturer_id) WHERE manufacturer_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS products_manufacturer_idx;
ALTER TABLE products DROP COLUMN IF EXISTS manufacturer_id;
DROP TABLE IF EXISTS manufacturers;

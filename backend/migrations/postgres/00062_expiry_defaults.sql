-- +goose Up
-- Receiving a delivery means typing an expiry for every line, read off a pack
-- that usually prints only a month and year. Two additive facts make that
-- faster without making the dates less trustworthy:
--
--   products.expiry_default (+ _months) -- how a product's new lots get their
--   expiry PRE-FILLED: MANUAL (type it, as before), MONTHS (received date + N
--   months, end of that month), or NONE (the goods do not expire). The server
--   only stores the setting; the Receive dialog computes the date and seeds
--   the field, and the person receiving can always type over it.
--
--   batches.expiry_source -- where a lot's date came from: ENTERED (typed, or
--   confirmed later), DEFAULT (the product's estimate, accepted unchanged), or
--   NONE (the date is the 2099-12-31 placeholder). A default nobody checked
--   would otherwise be indistinguishable from a date read off the pack, and
--   FEFO, the expiring-soon tile and every badge trust that date.
--
-- Every existing lot was typed, so ENTERED is the honest backfill; every
-- existing product keeps today's behaviour, so MANUAL.
ALTER TABLE products ADD COLUMN expiry_default TEXT NOT NULL DEFAULT 'MANUAL'
  CHECK (expiry_default IN ('MANUAL', 'MONTHS', 'NONE'));
ALTER TABLE products ADD COLUMN expiry_default_months INTEGER NOT NULL DEFAULT 0
  CHECK (expiry_default_months BETWEEN 0 AND 120);
ALTER TABLE batches ADD COLUMN expiry_source TEXT NOT NULL DEFAULT 'ENTERED'
  CHECK (expiry_source IN ('ENTERED', 'DEFAULT', 'NONE'));

-- +goose Down
ALTER TABLE batches DROP COLUMN IF EXISTS expiry_source;
ALTER TABLE products DROP COLUMN IF EXISTS expiry_default_months;
ALTER TABLE products DROP COLUMN IF EXISTS expiry_default;

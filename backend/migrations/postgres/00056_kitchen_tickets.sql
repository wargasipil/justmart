-- +goose Up
-- Kitchen tickets: per-line firing state + a preparation note.
--
-- fired_at is what makes a ticket incremental. A dine-in bill grows across
-- rounds, so "fire" must send only the lines added since the last fire —
-- reprinting the whole order every round would have the kitchen cook the
-- starters twice. NULL = not yet sent to the kitchen.
--
-- Firing is per LINE rather than per sale for the same reason: it is the lines,
-- not the bill, that are new.
ALTER TABLE sale_items ADD COLUMN fired_at TIMESTAMPTZ;

-- The cook-facing note: "no ice", "extra pedas", "well done". Deliberately free
-- text rather than a priced modifier system — a modifier that changes the price
-- is a different feature (it touches line totals, discounts and tier pricing),
-- and pretending a note is one would quietly get the money wrong. A note never
-- affects any amount.
ALTER TABLE sale_items ADD COLUMN kitchen_note TEXT NOT NULL DEFAULT '';

-- Partial index: the fire path only ever asks for the UNFIRED lines of one sale.
CREATE INDEX sale_items_unfired_idx ON sale_items(sale_id) WHERE fired_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS sale_items_unfired_idx;
ALTER TABLE sale_items DROP COLUMN IF EXISTS kitchen_note;
ALTER TABLE sale_items DROP COLUMN IF EXISTS fired_at;

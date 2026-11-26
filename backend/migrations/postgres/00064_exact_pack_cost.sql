-- +goose Up
-- A restock line is typed the way the supplier's faktur prices it: per PACK
-- (Rp 65.000 per box of 30). Until now only a per-BASE-unit cost was stored,
-- rounded (65.000 / 30 = 2.166,67 -> 2.167), and the line total was rebuilt
-- from it (50 box x 30 x 2.167 = 3.250.500 instead of 3.250.000). That error
-- reached the order total and, through it, the supplier's debt.
--
--   purchase_order_items.pack_cost_price -- GROSS cost of ONE of the line's
--   purchase unit (product_unit_id / unit_factor), exactly as typed. The line
--   gross is then qty-in-unit x pack_cost_price, exact. unit_cost_price stays,
--   derived (rounded) per base, for the readers that still want a per-base
--   figure. 0 = a line saved before this column: the old per-base math.
--
--   batches.cost_pack_price / cost_pack_factor -- the lot's NET cost (after
--   the line discount, PPN capitalized; the same basis as cost_price) for ONE
--   pack of cost_pack_factor base units, stamped at receive. Lets the product
--   page show "Rp 65.000 / box" instead of 2.167 x 30 = 65.010. 0 = not known
--   (older lots, manual / imported stock): readers derive from cost_price.
ALTER TABLE purchase_order_items ADD COLUMN pack_cost_price BIGINT NOT NULL DEFAULT 0;
ALTER TABLE batches ADD COLUMN cost_pack_price BIGINT NOT NULL DEFAULT 0;
ALTER TABLE batches ADD COLUMN cost_pack_factor BIGINT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE batches DROP COLUMN IF EXISTS cost_pack_factor;
ALTER TABLE batches DROP COLUMN IF EXISTS cost_pack_price;
ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS pack_cost_price;

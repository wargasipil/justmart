-- +goose Up
-- Dining tables + the order type, for restaurant mode.
--
-- A table belongs to a WAREHOUSE, which is this app's location concept (an
-- outlet's store IS its warehouse). That makes "the tables of this outlet" the
-- same scoping question as "the stock of this outlet", answered by the same
-- X-Warehouse-Id header, instead of inventing a parallel notion of site.
CREATE TABLE dining_tables (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  warehouse_id UUID NOT NULL REFERENCES warehouses(id),
  code         TEXT NOT NULL,
  name         TEXT NOT NULL DEFAULT '',
  area         TEXT NOT NULL DEFAULT '',
  seats        INTEGER NOT NULL DEFAULT 0 CHECK (seats >= 0),
  active       BOOLEAN NOT NULL DEFAULT TRUE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Codes are unique per outlet and only among LIVE tables, so an archived "T1"
-- doesn't block reusing the number when the floor is rearranged (mirrors the
-- partial-unique pattern already used for product units).
CREATE UNIQUE INDEX dining_tables_code_key ON dining_tables(warehouse_id, code) WHERE active;
CREATE INDEX dining_tables_warehouse_idx ON dining_tables(warehouse_id);

-- An open bill is just a DRAFT sale bound to a table. No new state machine: the
-- existing DRAFT -> COMPLETED | VOIDED flow already models "being built, then
-- settled or abandoned", which is exactly a table's life.
ALTER TABLE sales ADD COLUMN table_id UUID REFERENCES dining_tables(id);
-- '' = not a dine-in flow (every pre-existing sale, and every retail/pharmacy
-- sale forever). Left as a free string rather than a CHECK-ed enum so adding a
-- channel later (an aggregator, say) doesn't need a table rewrite on SQLite.
ALTER TABLE sales ADD COLUMN order_type TEXT NOT NULL DEFAULT '';
ALTER TABLE sales ADD COLUMN guest_count INTEGER NOT NULL DEFAULT 0 CHECK (guest_count >= 0);

-- ONE open bill per table, enforced by the database rather than by a read-then-
-- write check in the service: two waiters tapping the same table at the same
-- moment is the normal case during a rush, not a rare race.
CREATE UNIQUE INDEX sales_open_table_idx ON sales(table_id)
  WHERE table_id IS NOT NULL AND status = 'DRAFT';
CREATE INDEX sales_table_idx ON sales(table_id);

-- +goose Down
DROP INDEX IF EXISTS sales_table_idx;
DROP INDEX IF EXISTS sales_open_table_idx;
ALTER TABLE sales DROP COLUMN IF EXISTS guest_count;
ALTER TABLE sales DROP COLUMN IF EXISTS order_type;
ALTER TABLE sales DROP COLUMN IF EXISTS table_id;
DROP TABLE IF EXISTS dining_tables;

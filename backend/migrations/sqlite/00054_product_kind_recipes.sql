-- +goose Up
-- Mirror of postgres 00054_product_kind_recipes.sql (SQLite dialect). See the
-- postgres file for what product_kind means and why recipes are mode-independent.
-- UUID->TEXT (PKs filled by the Go create-callback), BIGINT->INTEGER,
-- TIMESTAMPTZ->DATETIME.
--
-- No table rebuild: the products add is an additive NOT NULL DEFAULT <constant>,
-- which SQLite's ALTER TABLE ADD COLUMN supports. The kind CHECK is therefore
-- NOT enforced by SQLite for this column (SQLite can only add a CHECK by
-- rebuilding the table, and rebuilding `products` — the most referenced table in
-- the schema — to constrain a service-validated enum is a bad trade). The
-- service validates product_kind on every write on both engines; Postgres keeps
-- the constraint as the backstop.
ALTER TABLE products ADD COLUMN product_kind TEXT NOT NULL DEFAULT 'STOCKED';

CREATE TABLE product_recipe_items (
    id                   TEXT PRIMARY KEY NOT NULL,
    product_id           TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    component_product_id TEXT NOT NULL REFERENCES products(id),
    qty_base             INTEGER NOT NULL CHECK (qty_base > 0),
    note                 TEXT NOT NULL DEFAULT '',
    created_at           DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at           DATETIME NOT NULL DEFAULT (datetime('now')),
    CHECK (product_id <> component_product_id)
);
CREATE UNIQUE INDEX product_recipe_items_product_component_key
  ON product_recipe_items(product_id, component_product_id);
CREATE INDEX product_recipe_items_component_idx ON product_recipe_items(component_product_id);

-- +goose Down
DROP TABLE IF EXISTS product_recipe_items;
ALTER TABLE products DROP COLUMN product_kind;

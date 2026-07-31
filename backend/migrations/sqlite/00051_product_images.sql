-- +goose Up
-- Mirror of postgres 00051_product_images.sql (SQLite dialect). UUID->TEXT (the
-- product_id is supplied by the caller, not generated), BYTEA->BLOB,
-- TIMESTAMPTZ->DATETIME. Two renditions per upload (original + thumb) — see the
-- postgres file for why they are separate columns and why thumb_data is NOT NULL.
--
-- No table rebuild needed: the new table is brand new, and the products column is
-- an additive NULLable ADD COLUMN, which SQLite's ALTER TABLE supports.
CREATE TABLE product_images (
    product_id   TEXT PRIMARY KEY NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    content_type TEXT NOT NULL,
    image_data   BLOB NOT NULL,
    thumb_data   BLOB NOT NULL,
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

ALTER TABLE products ADD COLUMN image_updated_at DATETIME;

-- +goose Down
ALTER TABLE products DROP COLUMN image_updated_at;
DROP TABLE IF EXISTS product_images;

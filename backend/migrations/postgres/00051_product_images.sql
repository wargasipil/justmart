-- +goose Up
-- Product pictures. Same shape and the same reasoning as user_avatars (00050):
-- the bytes live in their OWN table, not a column on `products`. GORM reads
-- products with SELECT * on every List/Get/Search/Resolve/POS-catalog load, so a
-- BYTEA on `products` would ride along on all of them — a 25-row product list
-- would pull megabytes to render 25 thumbnails.
--
-- TWO renditions per upload (the two-rendition HARD RULE in CLAUDE.md):
--   image_data — the ORIGINAL, bounded on the client (not the raw camera file)
--   thumb_data — a small square THUMB that the products list, the POS search
--                rows, and every other fast-load surface read instead.
-- thumb_data is NOT NULL: a row without a thumbnail would silently push those
-- surfaces back onto the original, which is the whole thing this prevents.
--
-- product_id is the PRIMARY KEY, so a product has AT MOST ONE picture and a
-- re-upload is an upsert rather than a second row. Making this a gallery later
-- means a new table with its own id + sort_order, not relaxing this key.
--
-- ON DELETE CASCADE because a picture has no meaning without its product.
-- Products are soft-deleted (active = false), so this only fires on a hard
-- delete — and an archived product deliberately keeps its picture.
CREATE TABLE product_images (
  product_id   UUID PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
  content_type TEXT  NOT NULL,
  image_data   BYTEA NOT NULL,
  thumb_data   BYTEA NOT NULL,
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Denormalized "has a picture, and how fresh" marker on the product row. NULL =
-- none. This is what lets ListProducts / SearchProducts / GetProduct tell the UI
-- whether to fetch bytes at all, and doubles as the cache-buster in the frontend
-- query key so a re-upload is picked up without a manual refresh. Kept in sync
-- by the product-image RPCs.
ALTER TABLE products ADD COLUMN image_updated_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE products DROP COLUMN IF EXISTS image_updated_at;
DROP TABLE IF EXISTS product_images;

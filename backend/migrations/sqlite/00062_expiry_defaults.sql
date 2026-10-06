-- +goose Up
-- Mirror of postgres 00062_expiry_defaults.sql (SQLite dialect). See that file
-- for what the three columns mean and why ENTERED / MANUAL are the backfill.
-- Plain ADD COLUMNs, so no table rebuild. The CHECKs the Postgres side carries
-- are left off here on purpose: SQLite refuses to DROP a column a CHECK refers
-- to, which would make the Down step fail. The service validates both values
-- before every write, on either engine.
ALTER TABLE products ADD COLUMN expiry_default TEXT NOT NULL DEFAULT 'MANUAL';
ALTER TABLE products ADD COLUMN expiry_default_months INTEGER NOT NULL DEFAULT 0;
ALTER TABLE batches ADD COLUMN expiry_source TEXT NOT NULL DEFAULT 'ENTERED';

-- +goose Down
ALTER TABLE batches DROP COLUMN expiry_source;
ALTER TABLE products DROP COLUMN expiry_default_months;
ALTER TABLE products DROP COLUMN expiry_default;

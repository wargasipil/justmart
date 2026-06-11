-- +goose Up
-- Mirror of postgres 00037 (SQLite dialect). Plain ADD COLUMN with constant
-- defaults — tx-safe, no table rebuild.
ALTER TABLE prescriptions ADD COLUMN biaya_jasa      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE prescriptions ADD COLUMN patient_age     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE prescriptions ADD COLUMN patient_weight  TEXT    NOT NULL DEFAULT '';
ALTER TABLE prescriptions ADD COLUMN patient_allergy TEXT    NOT NULL DEFAULT '';
ALTER TABLE sales         ADD COLUMN biaya_jasa      INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE sales         DROP COLUMN biaya_jasa;
ALTER TABLE prescriptions DROP COLUMN patient_allergy;
ALTER TABLE prescriptions DROP COLUMN patient_weight;
ALTER TABLE prescriptions DROP COLUMN patient_age;
ALTER TABLE prescriptions DROP COLUMN biaya_jasa;

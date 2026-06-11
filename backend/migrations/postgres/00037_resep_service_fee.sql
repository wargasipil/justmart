-- +goose Up
-- Pharmacy resep: per-prescription clinical info + service fee (biaya jasa), and
-- a sale-level biaya jasa snapshot that flows into the order total.
ALTER TABLE prescriptions ADD COLUMN biaya_jasa      BIGINT  NOT NULL DEFAULT 0;
ALTER TABLE prescriptions ADD COLUMN patient_age     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE prescriptions ADD COLUMN patient_weight  TEXT    NOT NULL DEFAULT '';
ALTER TABLE prescriptions ADD COLUMN patient_allergy TEXT    NOT NULL DEFAULT '';
ALTER TABLE sales         ADD COLUMN biaya_jasa      BIGINT  NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE sales         DROP COLUMN IF EXISTS biaya_jasa;
ALTER TABLE prescriptions DROP COLUMN IF EXISTS patient_allergy;
ALTER TABLE prescriptions DROP COLUMN IF EXISTS patient_weight;
ALTER TABLE prescriptions DROP COLUMN IF EXISTS patient_age;
ALTER TABLE prescriptions DROP COLUMN IF EXISTS biaya_jasa;

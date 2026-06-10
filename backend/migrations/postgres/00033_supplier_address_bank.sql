-- +goose Up
-- Supplier address ("alamat") + bank account ("data rekening": bank / no. rekening / atas nama).
ALTER TABLE suppliers
  ADD COLUMN address             TEXT NOT NULL DEFAULT '',
  ADD COLUMN bank_name           TEXT NOT NULL DEFAULT '',
  ADD COLUMN bank_account_number TEXT NOT NULL DEFAULT '',
  ADD COLUMN bank_account_holder TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE suppliers
  DROP COLUMN IF EXISTS address,
  DROP COLUMN IF EXISTS bank_name,
  DROP COLUMN IF EXISTS bank_account_number,
  DROP COLUMN IF EXISTS bank_account_holder;

-- +goose Up
-- Allow the dedicated restaurant-mode role WAITER (pramusaji) in users.role.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
  CHECK (role IN ('OWNER','PHARMACIST','CASHIER','APOTEKER','WAITER'));

-- +goose Down
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
  CHECK (role IN ('OWNER','PHARMACIST','CASHIER','APOTEKER'));

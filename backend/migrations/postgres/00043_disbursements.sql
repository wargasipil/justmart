-- +goose Up
-- Payment-integration disbursements: provider-agnostic money-out records (Xendit
-- now, Midtrans later). Created in-process by payroll (one per payslip); status
-- advances PENDING → PROCESSING → COMPLETED/FAILED via gateway webhooks. Money =
-- BIGINT minor units. `external_id` is our idempotency key.
--
-- Brand-new table, so the status CHECK is created fresh (no SQLite rebuild). The
-- two employees/payslips ALTERs are plain ADD COLUMN with a constant default and
-- no CHECK — SQLite-safe.

CREATE TABLE disbursements (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider       TEXT NOT NULL,
  external_id    TEXT NOT NULL,
  reference_type TEXT NOT NULL DEFAULT '',
  reference_id   TEXT NOT NULL DEFAULT '',
  channel_code   TEXT NOT NULL DEFAULT '',
  account_number TEXT NOT NULL DEFAULT '',
  account_holder TEXT NOT NULL DEFAULT '',
  amount         BIGINT NOT NULL DEFAULT 0,
  currency       TEXT NOT NULL DEFAULT 'IDR',
  status         TEXT NOT NULL DEFAULT 'PENDING'
                 CHECK (status IN ('PENDING','PROCESSING','COMPLETED','FAILED','VOIDED')),
  provider_ref   TEXT NOT NULL DEFAULT '',
  failure_reason TEXT NOT NULL DEFAULT '',
  created_by     UUID NOT NULL REFERENCES users(id),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at   TIMESTAMPTZ
);
CREATE UNIQUE INDEX disbursements_external_id_uidx ON disbursements(external_id);
CREATE INDEX disbursements_ref_idx ON disbursements(reference_type, reference_id);
CREATE INDEX disbursements_status_idx ON disbursements(status);
CREATE INDEX disbursements_provider_ref_idx ON disbursements(provider, provider_ref);

ALTER TABLE employees ADD COLUMN bank_channel_code TEXT NOT NULL DEFAULT '';
ALTER TABLE payslips  ADD COLUMN bank_channel_code TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE payslips  DROP COLUMN IF EXISTS bank_channel_code;
ALTER TABLE employees DROP COLUMN IF EXISTS bank_channel_code;
DROP TABLE IF EXISTS disbursements;

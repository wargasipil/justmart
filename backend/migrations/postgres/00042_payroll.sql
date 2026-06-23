-- +goose Up
-- Payroll (penggajian): a separate employee roster (optionally linked to a users
-- login account), monthly payroll runs, per-employee payslips with a flexible
-- component breakdown (earnings + BPJS + PPh21 deductions). Money = BIGINT minor
-- units (whole rupiah); statutory rates live in app_settings
-- (payroll_bpjs_config / payroll_pph21_config), not in the schema.
--
-- All tables are NEW here, so the CHECK constraints are created fresh (no ALTER).
-- NOTE for future changes: SQLite can't ALTER a CHECK in place, so adding a new
-- component kind/code or run status later needs a table-rebuild migration (see
-- sqlite/00035 / sqlite/00041). The `code` set is deliberately broad (MANUAL +
-- BONUS + every employer code) so new manual line types map to MANUAL without a
-- schema change.

CREATE TABLE employees (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code                 TEXT UNIQUE NOT NULL,
  name                 TEXT NOT NULL,
  user_id              UUID REFERENCES users(id),
  position             TEXT NOT NULL DEFAULT '',
  base_salary          BIGINT NOT NULL DEFAULT 0,
  commission_pct       INTEGER NOT NULL DEFAULT 0,
  npwp                 TEXT NOT NULL DEFAULT '',
  ptkp_status          TEXT NOT NULL DEFAULT 'TK0',
  bpjs_kes_no          TEXT NOT NULL DEFAULT '',
  bpjs_tk_no           TEXT NOT NULL DEFAULT '',
  bpjs_kes_enrolled    BOOLEAN NOT NULL DEFAULT FALSE,
  bpjs_jht_enrolled    BOOLEAN NOT NULL DEFAULT FALSE,
  bpjs_jp_enrolled     BOOLEAN NOT NULL DEFAULT FALSE,
  bpjs_jkk_enrolled    BOOLEAN NOT NULL DEFAULT FALSE,
  bpjs_jkm_enrolled    BOOLEAN NOT NULL DEFAULT FALSE,
  bank_name            TEXT NOT NULL DEFAULT '',
  bank_account_number  TEXT NOT NULL DEFAULT '',
  bank_account_holder  TEXT NOT NULL DEFAULT '',
  joined_at            DATE,
  active               BOOLEAN NOT NULL DEFAULT TRUE,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX employees_user_uidx ON employees(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX employees_active_idx ON employees(active);

CREATE TABLE employee_allowances (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
  label       TEXT NOT NULL,
  amount      BIGINT NOT NULL DEFAULT 0,
  taxable     BOOLEAN NOT NULL DEFAULT TRUE,
  active      BOOLEAN NOT NULL DEFAULT TRUE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX employee_allowances_emp_idx ON employee_allowances(employee_id);

CREATE TABLE payroll_no_counters (
  year     INTEGER PRIMARY KEY,
  last_seq INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE payroll_runs (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_no           TEXT UNIQUE,
  period_year      INTEGER NOT NULL,
  period_month     INTEGER NOT NULL CHECK (period_month BETWEEN 1 AND 12),
  status           TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','APPROVED','PAID','VOIDED')),
  note             TEXT NOT NULL DEFAULT '',
  total_gross      BIGINT NOT NULL DEFAULT 0,
  total_deductions BIGINT NOT NULL DEFAULT 0,
  total_net        BIGINT NOT NULL DEFAULT 0,
  created_by       UUID NOT NULL REFERENCES users(id),
  approved_at      TIMESTAMPTZ,
  paid_at          TIMESTAMPTZ,
  voided_at        TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- One live (non-voided) run per month; a voided run frees the slot for a redo.
CREATE UNIQUE INDEX payroll_runs_period_uidx ON payroll_runs(period_year, period_month) WHERE status <> 'VOIDED';
CREATE INDEX payroll_runs_status_idx ON payroll_runs(status);

CREATE TABLE payslips (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id              UUID NOT NULL REFERENCES payroll_runs(id) ON DELETE CASCADE,
  employee_id         UUID NOT NULL REFERENCES employees(id),
  employee_code       TEXT NOT NULL DEFAULT '',
  employee_name       TEXT NOT NULL DEFAULT '',
  position            TEXT NOT NULL DEFAULT '',
  npwp                TEXT NOT NULL DEFAULT '',
  ptkp_status         TEXT NOT NULL DEFAULT '',
  bank_name           TEXT NOT NULL DEFAULT '',
  bank_account_number TEXT NOT NULL DEFAULT '',
  bank_account_holder TEXT NOT NULL DEFAULT '',
  base_salary         BIGINT NOT NULL DEFAULT 0,
  commission_base     BIGINT NOT NULL DEFAULT 0,
  gross               BIGINT NOT NULL DEFAULT 0,
  total_deductions    BIGINT NOT NULL DEFAULT 0,
  net                 BIGINT NOT NULL DEFAULT 0,
  employer_cost       BIGINT NOT NULL DEFAULT 0,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (run_id, employee_id)
);
CREATE INDEX payslips_run_idx ON payslips(run_id);
CREATE INDEX payslips_employee_idx ON payslips(employee_id);

CREATE TABLE payslip_components (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  payslip_id       UUID NOT NULL REFERENCES payslips(id) ON DELETE CASCADE,
  kind             TEXT NOT NULL CHECK (kind IN ('EARNING','DEDUCTION','EMPLOYER_CONTRIB')),
  code             TEXT NOT NULL CHECK (code IN ('BASE','ALLOWANCE','COMMISSION','BONUS','BPJS_KES_EMP','BPJS_JHT_EMP','BPJS_JP_EMP','BPJS_KES_ER','BPJS_JHT_ER','BPJS_JP_ER','BPJS_JKK_ER','BPJS_JKM_ER','PPH21','MANUAL')),
  label            TEXT NOT NULL DEFAULT '',
  amount           BIGINT NOT NULL DEFAULT 0,
  taxable          BOOLEAN NOT NULL DEFAULT FALSE,
  system_generated BOOLEAN NOT NULL DEFAULT TRUE,
  overridden       BOOLEAN NOT NULL DEFAULT FALSE,
  sort_order       INTEGER NOT NULL DEFAULT 0,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX payslip_components_payslip_idx ON payslip_components(payslip_id);

-- +goose Down
DROP TABLE IF EXISTS payslip_components;
DROP TABLE IF EXISTS payslips;
DROP TABLE IF EXISTS payroll_runs;
DROP TABLE IF EXISTS payroll_no_counters;
DROP TABLE IF EXISTS employee_allowances;
DROP TABLE IF EXISTS employees;

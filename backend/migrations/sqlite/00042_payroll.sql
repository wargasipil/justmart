-- +goose Up
-- Mirror of postgres/00042_payroll.sql in the SQLite dialect. All tables are new
-- (CREATE TABLE), so this runs in a normal transaction — no table rebuild needed.
-- UUID PKs are TEXT, filled app-side by the GORM create-callback
-- (db.registerUUIDDefault); booleans are INTEGER (0/1); timestamps are DATETIME.

CREATE TABLE employees (
    id                  TEXT PRIMARY KEY NOT NULL,
    code                TEXT UNIQUE NOT NULL,
    name                TEXT NOT NULL,
    user_id             TEXT REFERENCES users(id),
    position            TEXT NOT NULL DEFAULT '',
    base_salary         INTEGER NOT NULL DEFAULT 0,
    commission_pct      INTEGER NOT NULL DEFAULT 0,
    npwp                TEXT NOT NULL DEFAULT '',
    ptkp_status         TEXT NOT NULL DEFAULT 'TK0',
    bpjs_kes_no         TEXT NOT NULL DEFAULT '',
    bpjs_tk_no          TEXT NOT NULL DEFAULT '',
    bpjs_kes_enrolled   INTEGER NOT NULL DEFAULT 0,
    bpjs_jht_enrolled   INTEGER NOT NULL DEFAULT 0,
    bpjs_jp_enrolled    INTEGER NOT NULL DEFAULT 0,
    bpjs_jkk_enrolled   INTEGER NOT NULL DEFAULT 0,
    bpjs_jkm_enrolled   INTEGER NOT NULL DEFAULT 0,
    bank_name           TEXT NOT NULL DEFAULT '',
    bank_account_number TEXT NOT NULL DEFAULT '',
    bank_account_holder TEXT NOT NULL DEFAULT '',
    joined_at           DATE,
    active              INTEGER NOT NULL DEFAULT 1,
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX employees_user_uidx ON employees(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX employees_active_idx ON employees(active);

CREATE TABLE employee_allowances (
    id          TEXT PRIMARY KEY NOT NULL,
    employee_id TEXT NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    label       TEXT NOT NULL,
    amount      INTEGER NOT NULL DEFAULT 0,
    taxable     INTEGER NOT NULL DEFAULT 1,
    active      INTEGER NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX employee_allowances_emp_idx ON employee_allowances(employee_id);

CREATE TABLE payroll_no_counters (
    year     INTEGER PRIMARY KEY NOT NULL,
    last_seq INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE payroll_runs (
    id               TEXT PRIMARY KEY NOT NULL,
    run_no           TEXT UNIQUE,
    period_year      INTEGER NOT NULL,
    period_month     INTEGER NOT NULL CHECK (period_month BETWEEN 1 AND 12),
    status           TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','APPROVED','PAID','VOIDED')),
    note             TEXT NOT NULL DEFAULT '',
    total_gross      INTEGER NOT NULL DEFAULT 0,
    total_deductions INTEGER NOT NULL DEFAULT 0,
    total_net        INTEGER NOT NULL DEFAULT 0,
    created_by       TEXT NOT NULL REFERENCES users(id),
    approved_at      DATETIME,
    paid_at          DATETIME,
    voided_at        DATETIME,
    created_at       DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at       DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX payroll_runs_period_uidx ON payroll_runs(period_year, period_month) WHERE status <> 'VOIDED';
CREATE INDEX payroll_runs_status_idx ON payroll_runs(status);

CREATE TABLE payslips (
    id                  TEXT PRIMARY KEY NOT NULL,
    run_id              TEXT NOT NULL REFERENCES payroll_runs(id) ON DELETE CASCADE,
    employee_id         TEXT NOT NULL REFERENCES employees(id),
    employee_code       TEXT NOT NULL DEFAULT '',
    employee_name       TEXT NOT NULL DEFAULT '',
    position            TEXT NOT NULL DEFAULT '',
    npwp                TEXT NOT NULL DEFAULT '',
    ptkp_status         TEXT NOT NULL DEFAULT '',
    bank_name           TEXT NOT NULL DEFAULT '',
    bank_account_number TEXT NOT NULL DEFAULT '',
    bank_account_holder TEXT NOT NULL DEFAULT '',
    base_salary         INTEGER NOT NULL DEFAULT 0,
    commission_base     INTEGER NOT NULL DEFAULT 0,
    gross               INTEGER NOT NULL DEFAULT 0,
    total_deductions    INTEGER NOT NULL DEFAULT 0,
    net                 INTEGER NOT NULL DEFAULT 0,
    employer_cost       INTEGER NOT NULL DEFAULT 0,
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (run_id, employee_id)
);
CREATE INDEX payslips_run_idx ON payslips(run_id);
CREATE INDEX payslips_employee_idx ON payslips(employee_id);

CREATE TABLE payslip_components (
    id               TEXT PRIMARY KEY NOT NULL,
    payslip_id       TEXT NOT NULL REFERENCES payslips(id) ON DELETE CASCADE,
    kind             TEXT NOT NULL CHECK (kind IN ('EARNING','DEDUCTION','EMPLOYER_CONTRIB')),
    code             TEXT NOT NULL CHECK (code IN ('BASE','ALLOWANCE','COMMISSION','BONUS','BPJS_KES_EMP','BPJS_JHT_EMP','BPJS_JP_EMP','BPJS_KES_ER','BPJS_JHT_ER','BPJS_JP_ER','BPJS_JKK_ER','BPJS_JKM_ER','PPH21','MANUAL')),
    label            TEXT NOT NULL DEFAULT '',
    amount           INTEGER NOT NULL DEFAULT 0,
    taxable          INTEGER NOT NULL DEFAULT 0,
    system_generated INTEGER NOT NULL DEFAULT 1,
    overridden       INTEGER NOT NULL DEFAULT 0,
    sort_order       INTEGER NOT NULL DEFAULT 0,
    created_at       DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX payslip_components_payslip_idx ON payslip_components(payslip_id);

-- +goose Down
DROP TABLE IF EXISTS payslip_components;
DROP TABLE IF EXISTS payslips;
DROP TABLE IF EXISTS payroll_runs;
DROP TABLE IF EXISTS payroll_no_counters;
DROP TABLE IF EXISTS employee_allowances;
DROP TABLE IF EXISTS employees;

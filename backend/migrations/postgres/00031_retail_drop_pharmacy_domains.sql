-- +goose Up
-- Retail conversion (justmart): drop the pharmacy-only domains —
-- prescriptions (Rx), e-Faktur tax invoices (NSFP pool), and BPJS claims —
-- together with the sale/customer columns that fed them. Keeps `customers.address`
-- (a generic retail field). Runs before the medicine->product rename (00032).

-- Sale columns that belong to the dropped domains.
DROP INDEX IF EXISTS sales_prescription_idx;
ALTER TABLE sales DROP COLUMN IF EXISTS prescription_id;
ALTER TABLE sales DROP COLUMN IF EXISTS tax_invoice_no;
ALTER TABLE sales DROP COLUMN IF EXISTS tax_invoice_code;
ALTER TABLE sales DROP COLUMN IF EXISTS tax_invoice_dpp;
ALTER TABLE sales DROP COLUMN IF EXISTS tax_invoice_ppn;
ALTER TABLE sales DROP COLUMN IF EXISTS tax_invoice_issued_at;

-- Customer pharmacy/tax columns (keep `address`).
DROP INDEX IF EXISTS customers_bpjs_idx;
ALTER TABLE customers DROP COLUMN IF EXISTS bpjs_no;
ALTER TABLE customers DROP COLUMN IF EXISTS npwp;

-- Dropped-domain tables (children first so FKs resolve).
DROP TABLE IF EXISTS prescription_items;
DROP TABLE IF EXISTS prescriptions;
DROP TABLE IF EXISTS rx_no_counters;
DROP TABLE IF EXISTS bpjs_claims;
DROP TABLE IF EXISTS nsfp_pool;

-- +goose Down
-- Recreate the dropped domains (structure only; previously-dropped data is gone).
ALTER TABLE customers ADD COLUMN bpjs_no TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN npwp    TEXT NOT NULL DEFAULT '';
CREATE INDEX customers_bpjs_idx ON customers(bpjs_no) WHERE bpjs_no <> '';

ALTER TABLE sales ADD COLUMN tax_invoice_no        TEXT;
ALTER TABLE sales ADD COLUMN tax_invoice_code      TEXT;
ALTER TABLE sales ADD COLUMN tax_invoice_dpp       BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sales ADD COLUMN tax_invoice_ppn       BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sales ADD COLUMN tax_invoice_issued_at TIMESTAMPTZ;

CREATE TABLE nsfp_pool (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code          TEXT NOT NULL UNIQUE,
  fiscal_year   INT  NOT NULL,
  imported_by   UUID NOT NULL REFERENCES users(id),
  imported_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  used_at       TIMESTAMPTZ,
  sale_id       UUID REFERENCES sales(id),
  CONSTRAINT nsfp_used_has_sale CHECK ((used_at IS NULL) = (sale_id IS NULL))
);
CREATE INDEX nsfp_unused_idx ON nsfp_pool(fiscal_year, code) WHERE used_at IS NULL;
CREATE INDEX nsfp_sale_idx   ON nsfp_pool(sale_id) WHERE sale_id IS NOT NULL;

CREATE TABLE bpjs_claims (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_id       UUID NOT NULL REFERENCES sales(id),
  customer_id   UUID NOT NULL REFERENCES customers(id),
  bpjs_no       TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'DRAFT'
    CHECK (status IN ('DRAFT','SUBMITTED','APPROVED','REJECTED','PAID')),
  amount        BIGINT NOT NULL DEFAULT 0,
  external_ref  TEXT NOT NULL DEFAULT '',
  note          TEXT NOT NULL DEFAULT '',
  submitted_at  TIMESTAMPTZ,
  resolved_at   TIMESTAMPTZ,
  created_by    UUID NOT NULL REFERENCES users(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX bpjs_claims_sale_idx     ON bpjs_claims(sale_id);
CREATE INDEX bpjs_claims_customer_idx ON bpjs_claims(customer_id);
CREATE INDEX bpjs_claims_status_idx   ON bpjs_claims(status);

CREATE TABLE rx_no_counters (
  year     INT PRIMARY KEY,
  last_seq INT NOT NULL DEFAULT 0
);

CREATE TABLE prescriptions (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  rx_no         TEXT UNIQUE,
  customer_id   UUID NOT NULL REFERENCES customers(id),
  issuer_name   TEXT NOT NULL,
  issued_at     DATE NOT NULL,
  expires_at    DATE NOT NULL,
  note          TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','VOIDED')),
  created_by    UUID NOT NULL REFERENCES users(id),
  branch_id     UUID,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX prescriptions_customer_idx ON prescriptions(customer_id);
CREATE INDEX prescriptions_status_idx   ON prescriptions(status);
CREATE INDEX prescriptions_expires_idx  ON prescriptions(expires_at);

-- NOTE: a full revert runs 00032's down first (products -> medicines), so at
-- this point the catalog table is `medicines` again.
CREATE TABLE prescription_items (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  prescription_id      UUID NOT NULL REFERENCES prescriptions(id) ON DELETE CASCADE,
  medicine_id          UUID NOT NULL REFERENCES medicines(id),
  prescribed_qty       INTEGER NOT NULL CHECK (prescribed_qty > 0),
  dispensed_qty        INTEGER NOT NULL DEFAULT 0 CHECK (dispensed_qty >= 0),
  dosage_instructions  TEXT NOT NULL DEFAULT '',
  note                 TEXT NOT NULL DEFAULT '',
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT prescription_items_dispensed_le_prescribed CHECK (dispensed_qty <= prescribed_qty)
);
CREATE INDEX prescription_items_rx_idx       ON prescription_items(prescription_id);
CREATE INDEX prescription_items_medicine_idx ON prescription_items(medicine_id);

ALTER TABLE sales ADD COLUMN prescription_id UUID REFERENCES prescriptions(id);
CREATE INDEX sales_prescription_idx ON sales(prescription_id) WHERE prescription_id IS NOT NULL;

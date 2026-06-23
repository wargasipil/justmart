-- +goose Up
-- Mirror of postgres/00043_disbursements.sql in the SQLite dialect. New table
-- (CREATE) + two plain ADD COLUMNs (no CHECK) → runs in a normal transaction, no
-- table rebuild. UUID PK is TEXT (filled app-side by the GORM create-callback).

CREATE TABLE disbursements (
    id             TEXT PRIMARY KEY NOT NULL,
    provider       TEXT NOT NULL,
    external_id    TEXT NOT NULL,
    reference_type TEXT NOT NULL DEFAULT '',
    reference_id   TEXT NOT NULL DEFAULT '',
    channel_code   TEXT NOT NULL DEFAULT '',
    account_number TEXT NOT NULL DEFAULT '',
    account_holder TEXT NOT NULL DEFAULT '',
    amount         INTEGER NOT NULL DEFAULT 0,
    currency       TEXT NOT NULL DEFAULT 'IDR',
    status         TEXT NOT NULL DEFAULT 'PENDING'
                   CHECK (status IN ('PENDING','PROCESSING','COMPLETED','FAILED','VOIDED')),
    provider_ref   TEXT NOT NULL DEFAULT '',
    failure_reason TEXT NOT NULL DEFAULT '',
    created_by     TEXT NOT NULL REFERENCES users(id),
    created_at     DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at     DATETIME NOT NULL DEFAULT (datetime('now')),
    completed_at   DATETIME
);
CREATE UNIQUE INDEX disbursements_external_id_uidx ON disbursements(external_id);
CREATE INDEX disbursements_ref_idx ON disbursements(reference_type, reference_id);
CREATE INDEX disbursements_status_idx ON disbursements(status);
CREATE INDEX disbursements_provider_ref_idx ON disbursements(provider, provider_ref);

ALTER TABLE employees ADD COLUMN bank_channel_code TEXT NOT NULL DEFAULT '';
ALTER TABLE payslips  ADD COLUMN bank_channel_code TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE payslips  DROP COLUMN bank_channel_code;
ALTER TABLE employees DROP COLUMN bank_channel_code;
DROP TABLE IF EXISTS disbursements;

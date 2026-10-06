-- +goose Up
-- Every correction (or confirmation) of a received lot's expiry, kept the way
-- product_prices keeps price changes: old value, new value, who, when, why.
--
-- audit_log already records THAT SetBatchExpiry was called, but not which lot
-- nor the dates, and for expiry the dates are the point: moving a date LATER
-- is exactly how expired stock goes back on sale, by accident or otherwise. A
-- reason is required on every row.
--
-- ON DELETE CASCADE: CancelReceipt deletes a provably untouched lot outright
-- (it ceases to exist rather than being offset), and its corrections go with it.
CREATE TABLE batch_expiry_changes (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  batch_id          UUID NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
  old_expiry_date   DATE NOT NULL,
  old_expiry_source TEXT NOT NULL,
  new_expiry_date   DATE NOT NULL,
  new_expiry_source TEXT NOT NULL,
  reason            TEXT NOT NULL,
  changed_by        UUID NOT NULL REFERENCES users(id),
  changed_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX batch_expiry_changes_batch_idx
  ON batch_expiry_changes(batch_id, changed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS batch_expiry_changes;

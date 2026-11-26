-- +goose Up
-- Edit a received delivery ("ubah penerimaan"). A delivery entered with a
-- mistake (wrong qty, batch number, expiry, faktur no) is corrected by
-- ReplaceReceipt: ONE transaction that cancels the old receipt (the same rules
-- and effects as CancelReceipt) and receives the corrected delivery as a NEW
-- receipt with its own RCV number. The old document stays, voided, so the RCV
-- sequence has no gap and the audit trail shows what was typed first.
--
--   purchase_receipts.replaced_by_id -- on a receipt voided by a replace, the
--   receipt that superseded it ("diganti oleh RCV-..."). NULL on a live
--   receipt and on one cancelled outright (Batalkan).
ALTER TABLE purchase_receipts ADD COLUMN replaced_by_id UUID NULL REFERENCES purchase_receipts(id);

-- +goose Down
ALTER TABLE purchase_receipts DROP COLUMN IF EXISTS replaced_by_id;

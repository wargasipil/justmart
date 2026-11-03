package batch

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// SetBatchExpiry corrects — or confirms — a received lot's expiry. It changes
// the expiry and its source and nothing else (UpdateBatch rewrites every field,
// so a caller holding only a date would zero the lot's cost).
//
// A reason is required and every change lands in batch_expiry_changes: moving a
// date LATER is exactly how expired stock goes back on sale, so who did it and
// why must be answerable. Re-saving a DEFAULT date unchanged is a real change —
// it turns an estimate into a date somebody read off the pack — so it is
// recorded as ENTERED rather than refused as a no-op.
//
// The receipt line's own copy of the expiry (purchase_receipt_items.expiry_date,
// shown on the restock order) follows the lot, so the two screens never
// disagree; the history row keeps what it used to say.
func (s *BatchService) SetBatchExpiry(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.SetBatchExpiryRequest],
) (*connect.Response[inventoryifacev1.SetBatchExpiryResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	batchID := strings.TrimSpace(req.Msg.BatchId)
	if batchID == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "batch.batch_required")
	}
	reason := strings.TrimSpace(req.Msg.Reason)
	if reason == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "batch.reason_required")
	}
	// An edit is always a person's decision, never an estimate.
	if req.Msg.ExpirySource == inventoryifacev1.ExpirySource_EXPIRY_SOURCE_DEFAULT {
		return nil, common.TokenError(connect.CodeInvalidArgument, "batch.expiry_source_invalid")
	}
	newSource := common.ExpirySourceFromWire(int32(req.Msg.ExpirySource))
	newExpiry, ok := common.LotExpiry(req.Msg.ExpiryDate, newSource)
	if !ok {
		return nil, common.TokenError(connect.CodeInvalidArgument, "batch.expiry_invalid")
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var b model.Batch
		if e := common.RowLock(tx).Where("id = ?", batchID).First(&b).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("batch not found"))
			}
			return connect.NewError(connect.CodeInternal, e)
		}
		if newSource == common.ExpirySourceNone {
			if e := s.assertMayHaveNoExpiry(ctx, tx, b.ProductID); e != nil {
				return e
			}
		}
		oldSource := b.ExpirySource
		if oldSource == "" {
			oldSource = common.ExpirySourceEntered
		}
		if b.ExpiryDate.Format(common.DateLayout) == newExpiry.Format(common.DateLayout) &&
			oldSource == newSource {
			return common.TokenError(connect.CodeInvalidArgument, "batch.expiry_unchanged")
		}

		if e := tx.Model(&model.Batch{}).Where("id = ?", b.ID).Updates(map[string]any{
			"expiry_date":   newExpiry,
			"expiry_source": newSource,
		}).Error; e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}
		if e := tx.Model(&model.PurchaseReceiptItem{}).Where("batch_id = ?", b.ID).
			Update("expiry_date", newExpiry).Error; e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}
		if e := tx.Create(&model.BatchExpiryChange{
			BatchID:         b.ID,
			OldExpiryDate:   b.ExpiryDate,
			OldExpirySource: oldSource,
			NewExpiryDate:   newExpiry,
			NewExpirySource: newSource,
			Reason:          reason,
			ChangedBy:       caller.UserID,
			ChangedAt:       time.Now(),
		}).Error; e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	b, err := s.load(ctx, batchID)
	if err != nil {
		return nil, err
	}
	qty, err := common.BatchCurrentQty(ctx, s.db, b.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&inventoryifacev1.SetBatchExpiryResponse{Batch: batchToProto(b, qty)}), nil
}

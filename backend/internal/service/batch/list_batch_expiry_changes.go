package batch

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ListBatchExpiryChanges is one lot's history of expiry corrections, newest
// first, paginated like every List*. changed_by is a user id for the client to
// resolve (ResolveUsers) — names are never denormalized onto history rows.
func (s *BatchService) ListBatchExpiryChanges(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListBatchExpiryChangesRequest],
) (*connect.Response[inventoryifacev1.ListBatchExpiryChangesResponse], error) {
	batchID := strings.TrimSpace(req.Msg.BatchId)
	if batchID == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "batch.batch_required")
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)

	base := func() *gorm.DB {
		return s.db.WithContext(ctx).Model(&model.BatchExpiryChange{}).Where("batch_id = ?", batchID)
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.BatchExpiryChange
	// id DESC is the tiebreaker: two corrections in the same instant must not
	// swap places between pages.
	if err := base().Order("changed_at DESC, id DESC").
		Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*inventoryifacev1.BatchExpiryChange, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		out = append(out, &inventoryifacev1.BatchExpiryChange{
			Id:              r.ID,
			BatchId:         r.BatchID,
			OldExpiryDate:   r.OldExpiryDate.Format(common.DateLayout),
			OldExpirySource: inventoryifacev1.ExpirySource(common.ExpirySourceToWire(r.OldExpirySource)),
			NewExpiryDate:   r.NewExpiryDate.Format(common.DateLayout),
			NewExpirySource: inventoryifacev1.ExpirySource(common.ExpirySourceToWire(r.NewExpirySource)),
			Reason:          r.Reason,
			ChangedBy:       r.ChangedBy,
			ChangedAt:       r.ChangedAt.Unix(),
		})
	}
	return connect.NewResponse(&inventoryifacev1.ListBatchExpiryChangesResponse{
		Changes: out,
		Total:   int32(total),
	}), nil
}

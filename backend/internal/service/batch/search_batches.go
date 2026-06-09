package batch

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *BatchService) SearchBatches(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.SearchBatchesRequest],
) (*connect.Response[inventoryifacev1.SearchBatchesResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	query := strings.TrimSpace(req.Msg.Query)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := s.db.WithContext(ctx).
		Table("batches AS b").
		Joins("JOIN products AS m ON m.id = b.product_id").
		Order("b.expiry_date ASC").
		Limit(limit).
		Select("b.*")
	if req.Msg.ProductId != "" {
		q = q.Where("b.product_id = ?", req.Msg.ProductId)
	}
	if query != "" {
		pattern := "%" + query + "%"
		q = q.Where("b.batch_number "+common.LikeOp(q)+" ? OR m.name "+common.LikeOp(q)+" ? OR m.sku "+common.LikeOp(q)+" ?", pattern, pattern, pattern)
	}
	var rows []model.Batch
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Batch, 0, len(rows))
	for _, r := range rows {
		qty, err := common.BatchQtyInWarehouse(ctx, s.db, r.ID, warehouseID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		out = append(out, batchToProto(&r, qty))
	}
	return connect.NewResponse(&inventoryifacev1.SearchBatchesResponse{Batches: out}), nil
}

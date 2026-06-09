package batch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

func (s *BatchService) UpdateBatch(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UpdateBatchRequest],
) (*connect.Response[inventoryifacev1.UpdateBatchResponse], error) {
	batch, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{
		"batch_number": strings.TrimSpace(req.Msg.BatchNumber),
		"cost_price":   req.Msg.CostPrice,
	}
	if req.Msg.SupplierId != "" {
		updates["supplier_id"] = req.Msg.SupplierId
	}
	if req.Msg.ExpiryDate != "" {
		expiry, err := time.Parse(common.DateLayout, req.Msg.ExpiryDate)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("expiry_date must be YYYY-MM-DD: %w", err))
		}
		updates["expiry_date"] = expiry
	}
	if req.Msg.ReceivedAt != "" {
		received, err := time.Parse(common.DateLayout, req.Msg.ReceivedAt)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("received_at must be YYYY-MM-DD: %w", err))
		}
		updates["received_at"] = received
	}

	if err := s.db.WithContext(ctx).Model(batch).Updates(updates).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	batch, err = s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	qty, err := common.BatchCurrentQty(ctx, s.db, batch.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&inventoryifacev1.UpdateBatchResponse{Batch: batchToProto(batch, qty)}), nil
}

package batch

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/service/common"
)

func (s *BatchService) GetBatch(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetBatchRequest],
) (*connect.Response[inventoryifacev1.GetBatchResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	batch, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	qty, err := common.BatchQtyInWarehouse(ctx, s.db, batch.ID, warehouseID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&inventoryifacev1.GetBatchResponse{Batch: batchToProto(batch, qty)}), nil
}

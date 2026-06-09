package stocktake

import (
	"context"

	"connectrpc.com/connect"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
)

func (s *StocktakeService) AddBatchesToSession(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.AddBatchesToSessionRequest],
) (*connect.Response[stocktakeifacev1.AddBatchesToSessionResponse], error) {
	added, skipped, err := s.addBatches(ctx, req.Msg.SessionId, req.Msg.BatchIds)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&stocktakeifacev1.AddBatchesToSessionResponse{
		AddedCount:   added,
		SkippedCount: skipped,
	}), nil
}

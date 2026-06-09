package stocktake

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *StocktakeService) AddAllInStockBatches(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.AddAllInStockBatchesRequest],
) (*connect.Response[stocktakeifacev1.AddAllInStockBatchesResponse], error) {
	// Resolve the session's warehouse so we only seed batches in stock THERE.
	var session model.StocktakeSession
	if err := s.db.WithContext(ctx).Where("id = ?", req.Msg.SessionId).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("stocktake not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	whID := common.Deref(session.WarehouseID)

	// Find every batch with current qty > 0 in this warehouse.
	type batchRow struct {
		BatchID string `gorm:"column:batch_id"`
	}
	var rows []batchRow
	err := s.db.WithContext(ctx).
		Table("batches b").
		Select("b.id AS batch_id").
		Joins("LEFT JOIN stock_movements m ON m.batch_id = b.id AND m.warehouse_id = ?", whID).
		Group("b.id").
		Having("COALESCE(SUM(m.qty), 0) > 0").
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.BatchID)
	}
	added, skipped, err := s.addBatches(ctx, req.Msg.SessionId, ids)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&stocktakeifacev1.AddAllInStockBatchesResponse{
		AddedCount:   added,
		SkippedCount: skipped,
	}), nil
}

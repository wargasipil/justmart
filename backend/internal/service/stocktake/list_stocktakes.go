package stocktake

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *StocktakeService) ListStocktakes(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.ListStocktakesRequest],
) (*connect.Response[stocktakeifacev1.ListStocktakesResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		// Scope to the caller's active warehouse (header-driven, like ListMovements).
		q = q.Where("warehouse_id = ?", warehouseID)
		if st := strings.TrimSpace(strings.ToUpper(req.Msg.Status)); st != "" {
			q = q.Where("status = ?", st)
		}
		return q
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.StocktakeSession{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.StocktakeSession
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.StocktakeSession{})).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*stocktakeifacev1.StocktakeSession, 0, len(rows))
	for i := range rows {
		hydrated, err := s.hydrateSession(ctx, &rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, hydrated)
	}
	return connect.NewResponse(&stocktakeifacev1.ListStocktakesResponse{
		Sessions: out,
		Total:    int32(total),
	}), nil
}

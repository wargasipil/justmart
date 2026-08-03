package stocktake

import (
	"context"

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
	// Shared with GetStocktakeSummary — see applyStocktakeFilters in helpers.go.
	// Warehouse scoping is header-driven, like ListMovements.
	filters := sessionFilterArgs{
		WarehouseID: warehouseID,
		Status:      req.Msg.Status,
		FromUnix:    req.Msg.FromUnix,
		ToUnix:      req.Msg.ToUnix,
		DateField:   req.Msg.DateField,
	}
	applyFilters := func(q *gorm.DB) *gorm.DB { return applyStocktakeFilters(q, filters) }
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

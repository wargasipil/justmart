package stocktake

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// GetStocktakeSummary aggregates every session matching the list's filters —
// the stat row above the table, not a sum of the page on screen (that would
// change meaning as the user pages).
//
// It honors ListStocktakes' date range through the shared applyStocktakeFilters,
// so the tiles and the rows under them always describe the same sessions. It
// deliberately does NOT take a `status`: the figures ARE the status breakdown,
// so narrowing by one would zero out three of the four.
func (s *StocktakeService) GetStocktakeSummary(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.GetStocktakeSummaryRequest],
) (*connect.Response[stocktakeifacev1.GetStocktakeSummaryResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	filters := sessionFilterArgs{
		WarehouseID: warehouseID,
		FromUnix:    req.Msg.FromUnix,
		ToUnix:      req.Msg.ToUnix,
		DateField:   req.Msg.DateField,
	}

	var counts struct {
		Draft     int32 `gorm:"column:draft"`
		Completed int32 `gorm:"column:completed"`
		Voided    int32 `gorm:"column:voided"`
	}
	countQ := s.db.WithContext(ctx).
		Table("stocktake_sessions").
		Select(`COUNT(*) FILTER (WHERE status = ?) AS draft,
		        COUNT(*) FILTER (WHERE status = ?) AS completed,
		        COUNT(*) FILTER (WHERE status = ?) AS voided`,
			stocktakeStatusDraft, stocktakeStatusCompleted, stocktakeStatusVoided)
	if err := applyStocktakeFilters(countQ, filters).Scan(&counts).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Drift booked as movements: only COMPLETED sessions wrote any, so a DRAFT's
	// in-progress variances don't inflate the figure. Matching sessions are an
	// id sub-select rather than a join — created_at lives on both tables, so
	// unqualified filter columns would be ambiguous.
	completedFilters := filters
	completedFilters.Status = stocktakeStatusCompleted
	completedIDs := applyStocktakeFilters(
		s.db.WithContext(ctx).Table("stocktake_sessions").Select("id"), completedFilters)

	var varianceLines int64
	if err := s.db.WithContext(ctx).
		Table("stocktake_lines").
		Where("session_id IN (?)", completedIDs).
		Where("counted_qty IS NOT NULL AND counted_qty <> expected_qty").
		Count(&varianceLines).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Read the newest completed row rather than MAX(completed_at): a bare
	// aggregate over a nullable timestamp scans differently per engine.
	var lastCompletedAt int64
	var last model.StocktakeSession
	err = applyStocktakeFilters(s.db.WithContext(ctx).Model(&model.StocktakeSession{}), completedFilters).
		Where("completed_at IS NOT NULL").
		Order("completed_at DESC, id DESC").
		First(&last).Error
	switch {
	case err == nil:
		if last.CompletedAt != nil {
			lastCompletedAt = last.CompletedAt.Unix()
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		// Never counted in this range — 0 is the documented "never" sentinel.
	default:
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&stocktakeifacev1.GetStocktakeSummaryResponse{
		DraftCount:      counts.Draft,
		CompletedCount:  counts.Completed,
		VoidedCount:     counts.Voided,
		VarianceLines:   int32(varianceLines),
		LastCompletedAt: lastCompletedAt,
	}), nil
}

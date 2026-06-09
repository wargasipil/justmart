package sale

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SaleService) GetTodaySnapshot(
	ctx context.Context,
	req *connect.Request[posifacev1.GetTodaySnapshotRequest],
) (*connect.Response[posifacev1.GetTodaySnapshotResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, werr := common.ResolveWarehouse(ctx, s.db, caller)
	if werr != nil {
		return nil, connect.NewError(connect.CodeInternal, werr)
	}

	// Optional cashier filter — non-OWNER callers may only request their own.
	cashierID := req.Msg.CashierUserId
	if cashierID != "" && caller.Role != "OWNER" && cashierID != caller.UserID {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("can only request own snapshot"))
	}

	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	applySaleFilters := func(q *gorm.DB, alias string) *gorm.DB {
		q = q.Where(alias+"status = ?", saleStatusCompleted).
			Where(alias+"warehouse_id = ?", warehouseID).
			Where(alias+"completed_at >= ?", dayStart)
		if cashierID != "" {
			q = q.Where(alias+"cashier_user_id = ?", cashierID)
		}
		return q
	}

	var revenue int64
	if err := applySaleFilters(s.db.WithContext(ctx).Model(&model.Sale{}), "").
		Select("COALESCE(SUM(total), 0)").
		Scan(&revenue).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var saleCount int64
	if err := applySaleFilters(s.db.WithContext(ctx).Model(&model.Sale{}), "").
		Count(&saleCount).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var itemsSold int64
	if err := applySaleFilters(
		s.db.WithContext(ctx).Table("sale_items si").Joins("JOIN sales s ON s.id = si.sale_id"),
		"s.",
	).Select("COALESCE(SUM(si.base_qty), 0)").
		Scan(&itemsSold).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	type topRow struct {
		ProductID string
		Qty       int64
	}
	var top topRow
	_ = applySaleFilters(
		s.db.WithContext(ctx).Table("sale_items si").Joins("JOIN sales s ON s.id = si.sale_id"),
		"s.",
	).Select("si.product_id AS product_id, SUM(si.base_qty) AS qty").
		Group("si.product_id").
		Order("qty DESC").
		Limit(1).
		Scan(&top).Error

	var lastSaleUnix int64
	if err := applySaleFilters(s.db.WithContext(ctx).Model(&model.Sale{}), "").
		Select("COALESCE(" + common.EpochExpr(s.db, "MAX(completed_at)") + ", 0)").
		Scan(&lastSaleUnix).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&posifacev1.GetTodaySnapshotResponse{
		Revenue:       revenue,
		SaleCount:     saleCount,
		ItemsSold:     itemsSold,
		TopProductId:  top.ProductID,
		TopProductQty: top.Qty,
		LastSaleUnix:  lastSaleUnix,
	}), nil
}

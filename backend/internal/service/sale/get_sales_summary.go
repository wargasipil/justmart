package sale

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// GetSalesSummary aggregates over the SAME filters as ListSales across ALL
// matching rows (not one page).
func (s *SaleService) GetSalesSummary(
	ctx context.Context,
	req *connect.Request[posifacev1.GetSalesSummaryRequest],
) (*connect.Response[posifacev1.GetSalesSummaryResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	scope := func() *gorm.DB {
		return s.applySaleFilters(s.db.WithContext(ctx).Model(&model.Sale{}),
			warehouseID, req.Msg.FromUnix, req.Msg.ToUnix, req.Msg.Status, req.Msg.Query)
	}

	var saleCount int64
	if err := scope().Count(&saleCount).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var revenue int64
	if err := scope().Select("COALESCE(SUM(total), 0)").Scan(&revenue).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// items_sold = SUM(base_qty) over sale_items whose sale matches the same filters.
	idSub := s.applySaleFilters(s.db.WithContext(ctx).Model(&model.Sale{}).Select("id"),
		warehouseID, req.Msg.FromUnix, req.Msg.ToUnix, req.Msg.Status, req.Msg.Query)
	var itemsSold int64
	if err := s.db.WithContext(ctx).
		Table("sale_items").
		Where("sale_id IN (?)", idSub).
		Select("COALESCE(SUM(base_qty), 0)").
		Scan(&itemsSold).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&posifacev1.GetSalesSummaryResponse{
		SaleCount: saleCount,
		ItemsSold: itemsSold,
		Revenue:   revenue,
	}), nil
}

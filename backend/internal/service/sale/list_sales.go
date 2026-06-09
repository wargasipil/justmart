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

func (s *SaleService) ListSales(
	ctx context.Context,
	req *connect.Request[posifacev1.ListSalesRequest],
) (*connect.Response[posifacev1.ListSalesResponse], error) {
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
		return s.applySaleFilters(q, warehouseID, req.Msg.FromUnix, req.Msg.ToUnix, req.Msg.Status, req.Msg.Query)
	}

	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Sale{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.Sale
	if err := applyFilters(s.db.WithContext(ctx).Preload("Items")).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*posifacev1.Sale, 0, len(rows))
	for i := range rows {
		out = append(out, saleToProto(&rows[i]))
	}
	if err := s.enrichSaleNames(ctx, out); err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.ListSalesResponse{
		Sales: out,
		Total: int32(total),
	}), nil
}

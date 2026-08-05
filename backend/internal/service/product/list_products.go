package product

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *ProductService) ListProducts(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductsRequest],
) (*connect.Response[inventoryifacev1.ListProductsResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	filters, err := parseProductFilters(ctx, s.db, caller,
		req.Msg.IncludeInactive, req.Msg.OnlyArchived, req.Msg.Query, req.Msg.OpnameBefore)
	if err != nil {
		return nil, err
	}

	var total int64
	if err := filters.apply(s.db.WithContext(ctx).Model(&model.Product{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.Product
	if err := filters.apply(s.db.WithContext(ctx).Model(&model.Product{})).
		Order("name").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Product, 0, len(rows))
	for i := range rows {
		out = append(out, productToProto(&rows[i]))
	}
	if err := s.enrichStock(ctx, caller, out); err != nil {
		return nil, err
	}
	if err := s.enrichLastStocktake(ctx, caller, out); err != nil {
		return nil, err
	}
	if err := s.enrichLastRestock(ctx, caller, out); err != nil {
		return nil, err
	}
	if err := s.attachUnits(ctx, out); err != nil {
		return nil, err
	}
	redactCost(caller, out)
	return connect.NewResponse(&inventoryifacev1.ListProductsResponse{
		Products: out,
		Total:    int32(total),
	}), nil
}

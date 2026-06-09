package product

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *ProductService) SearchProducts(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.SearchProductsRequest],
) (*connect.Response[inventoryifacev1.SearchProductsResponse], error) {
	query := strings.TrimSpace(req.Msg.Query)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := s.db.WithContext(ctx).Order("name").Limit(limit)
	if !req.Msg.IncludeInactive {
		q = q.Where("active = ?", true)
	}
	if query != "" {
		pattern := "%" + query + "%"
		q = q.Where("name "+common.LikeOp(q)+" ? OR sku "+common.LikeOp(q)+" ?", pattern, pattern)
	}
	var rows []model.Product
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Product, 0, len(rows))
	for i := range rows {
		out = append(out, productToProto(&rows[i]))
	}
	if err := s.attachUnits(ctx, out); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.SearchProductsResponse{Products: out}), nil
}

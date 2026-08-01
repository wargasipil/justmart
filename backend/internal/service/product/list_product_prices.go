package product

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ListProductPrices is the legacy BASE-unit-only price history, superseded by
// ListProductUnitPrices (which covers every unit). Kept for back-compat; paged
// on the same terms, since the history grows a row per price edit.
func (s *ProductService) ListProductPrices(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductPricesRequest],
) (*connect.Response[inventoryifacev1.ListProductPricesResponse], error) {
	if req.Msg.ProductId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_id required"))
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)

	// One filter closure feeds both the count and the page, so the two can't drift.
	applyFilters := func(q *gorm.DB) *gorm.DB {
		return q.Model(&model.ProductPrice{}).Where("product_id = ?", req.Msg.ProductId)
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.ProductPrice
	// id breaks the effective_from tie (a seed + an immediate edit share a
	// timestamp), so paging can't drop or repeat a row.
	if err := applyFilters(s.db.WithContext(ctx)).
		Order("effective_from DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.ProductPrice, 0, len(rows))
	for _, r := range rows {
		out = append(out, productPriceToProto(&r))
	}
	return connect.NewResponse(&inventoryifacev1.ListProductPricesResponse{
		Prices: out,
		Total:  int32(total),
	}), nil
}

package productdiscount

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ListProductDiscounts returns one page of a product's discounts, newest first.
// Ordered by created_at with the id as a tiebreak so rows can't shuffle between
// pages when several are created in the same second.
func (s *ProductDiscountService) ListProductDiscounts(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductDiscountsRequest],
) (*connect.Response[inventoryifacev1.ListProductDiscountsResponse], error) {
	productID := strings.TrimSpace(req.Msg.ProductId)
	if productID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_id required"))
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)

	// One filter closure feeds both the count and the page, so the two can't drift.
	applyFilters := func(q *gorm.DB) *gorm.DB {
		return q.Model(&model.ProductDiscount{}).Where("product_id = ?", productID)
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.ProductDiscount
	if err := applyFilters(s.db.WithContext(ctx)).
		Order("created_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.ProductDiscount, len(rows))
	for i := range rows {
		out[i] = toProto(&rows[i])
	}
	return connect.NewResponse(&inventoryifacev1.ListProductDiscountsResponse{
		Discounts: out,
		Total:     int32(total),
	}), nil
}

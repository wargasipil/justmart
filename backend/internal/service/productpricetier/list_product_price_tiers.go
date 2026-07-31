package productpricetier

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

// ListProductPriceTiers returns one page of tiers for a product, ordered the way
// the Grosir card renders them: base unit first, then larger units by factor, and
// within a unit ascending by threshold. That order is stable, so a unit's ladder
// stays contiguous and only splits at a page boundary.
//
// NOTE: this is the ADMIN read. POS never calls it — it reads whole ladders off
// the `price_tiers` field embedded on Product (attachPriceTiers), which stays
// unpaginated on purpose: tier resolution needs every rung to pick the cheapest
// qualifying one, and a partial ladder would silently mis-price a sale.
func (s *ProductPriceTierService) ListProductPriceTiers(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductPriceTiersRequest],
) (*connect.Response[inventoryifacev1.ListProductPriceTiersResponse], error) {
	productID := strings.TrimSpace(req.Msg.ProductId)
	if productID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_id required"))
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)

	// One filter closure feeds both the count and the page, so the two can't drift.
	applyFilters := func(q *gorm.DB) *gorm.DB {
		return q.Model(&model.ProductPriceTier{}).
			Joins("JOIN product_units pu ON pu.id = product_price_tiers.product_unit_id").
			Where("product_price_tiers.product_id = ?", productID)
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.ProductPriceTier
	if err := applyFilters(s.db.WithContext(ctx)).
		Order("pu.is_base DESC, pu.factor ASC, product_price_tiers.min_qty ASC").
		Offset(offset).Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.ProductPriceTier, len(rows))
	for i := range rows {
		out[i] = toProto(&rows[i])
	}
	return connect.NewResponse(&inventoryifacev1.ListProductPriceTiersResponse{
		Tiers: out,
		Total: int32(total),
	}), nil
}

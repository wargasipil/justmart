package productpricetier

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// ListProductPriceTiers returns every tier for one product, ordered the way the
// Grosir tab renders them: base unit first, then larger units by factor, and
// within a unit ascending by threshold. A product has only a handful, so this is
// unpaginated (mirrors ListProductDiscounts).
func (s *ProductPriceTierService) ListProductPriceTiers(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductPriceTiersRequest],
) (*connect.Response[inventoryifacev1.ListProductPriceTiersResponse], error) {
	productID := strings.TrimSpace(req.Msg.ProductId)
	if productID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_id required"))
	}
	var rows []model.ProductPriceTier
	if err := s.db.WithContext(ctx).
		Model(&model.ProductPriceTier{}).
		Joins("JOIN product_units pu ON pu.id = product_price_tiers.product_unit_id").
		Where("product_price_tiers.product_id = ?", productID).
		Order("pu.is_base DESC, pu.factor ASC, product_price_tiers.min_qty ASC").
		Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.ProductPriceTier, len(rows))
	for i := range rows {
		out[i] = toProto(&rows[i])
	}
	return connect.NewResponse(&inventoryifacev1.ListProductPriceTiersResponse{Tiers: out}), nil
}

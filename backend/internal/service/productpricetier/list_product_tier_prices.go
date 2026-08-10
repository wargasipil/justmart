package productpricetier

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ListProductTierPrices returns one page of the grosir price history for a
// product — every rung of every unit, newest-first within a rung. Like
// ListProductUnitPrices this grows a row per price edit, so it is genuinely
// paginated rather than merely conforming.
func (s *ProductPriceTierService) ListProductTierPrices(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductTierPricesRequest],
) (*connect.Response[inventoryifacev1.ListProductTierPricesResponse], error) {
	if req.Msg.ProductId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_id required"))
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)

	// One filter closure feeds both the count and the page, so the two can't drift.
	// The column is qualified because the page below joins product_units, which
	// carries a product_id of its own.
	applyFilters := func(q *gorm.DB) *gorm.DB {
		return q.Model(&model.ProductTierPrice{}).
			Where("product_tier_prices.product_id = ?", req.Msg.ProductId)
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	type row struct {
		ID            string     `gorm:"column:id"`
		ProductID     string     `gorm:"column:product_id"`
		ProductUnitID string     `gorm:"column:product_unit_id"`
		UnitName      string     `gorm:"column:unit_name"`
		MinQty        int32      `gorm:"column:min_qty"`
		Price         int64      `gorm:"column:price"`
		EffectiveFrom time.Time  `gorm:"column:effective_from"`
		EffectiveTo   *time.Time `gorm:"column:effective_to"`
		ChangedBy     *string    `gorm:"column:changed_by"`
	}
	var rows []row
	err := applyFilters(s.db.WithContext(ctx)).
		// The unit join is for ORDERING only (base first, then ascending factor —
		// the order the Grosir card renders); unit_name is read from the row's own
		// snapshot so a later rename doesn't rewrite history. Units are deactivated
		// rather than deleted, so the join never drops a row.
		Joins("JOIN product_units pu ON pu.id = product_tier_prices.product_unit_id").
		Select(`product_tier_prices.id, product_tier_prices.product_id,
		        product_tier_prices.product_unit_id, product_tier_prices.unit_name,
		        product_tier_prices.min_qty, product_tier_prices.price,
		        product_tier_prices.effective_from, product_tier_prices.effective_to,
		        product_tier_prices.changed_by`).
		// id breaks the effective_from tie: a create and an immediate edit can share
		// a timestamp, and an unstable order would let a row repeat on one page and
		// vanish from the next.
		Order(`pu.is_base DESC, pu.factor ASC, product_tier_prices.min_qty ASC,
		       product_tier_prices.effective_from DESC, product_tier_prices.id DESC`).
		Offset(offset).Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*inventoryifacev1.ProductTierPrice, 0, len(rows))
	for _, r := range rows {
		p := &inventoryifacev1.ProductTierPrice{
			Id:            r.ID,
			ProductId:     r.ProductID,
			ProductUnitId: r.ProductUnitID,
			UnitName:      r.UnitName,
			MinQty:        r.MinQty,
			Price:         r.Price,
			EffectiveFrom: r.EffectiveFrom.Unix(),
		}
		if r.EffectiveTo != nil {
			p.EffectiveTo = r.EffectiveTo.Unix()
		}
		if r.ChangedBy != nil {
			p.ChangedBy = *r.ChangedBy
		}
		out = append(out, p)
	}
	return connect.NewResponse(&inventoryifacev1.ListProductTierPricesResponse{
		Prices: out,
		Total:  int32(total),
	}), nil
}

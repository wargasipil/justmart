package product

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// ListProductUnitPrices returns one page of the per-unit sell-price history for a
// product, joined to the unit name and ordered base-first then newest-first. This
// history grows without bound (a row per price edit per unit), so it is the one
// product-detail read that genuinely needs paging rather than merely conforming.
func (s *ProductService) ListProductUnitPrices(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductUnitPricesRequest],
) (*connect.Response[inventoryifacev1.ListProductUnitPricesResponse], error) {
	if req.Msg.ProductId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_id required"))
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)

	// One filter closure feeds both the count and the page, so the two can't drift.
	applyFilters := func(q *gorm.DB) *gorm.DB {
		return q.Table("product_unit_prices mup").
			Joins("JOIN product_units mu ON mu.id = mup.product_unit_id").
			Where("mu.product_id = ?", req.Msg.ProductId)
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	type row struct {
		ID            string     `gorm:"column:id"`
		ProductUnitID string     `gorm:"column:product_unit_id"`
		UnitName      string     `gorm:"column:unit_name"`
		UnitSellPrice int64      `gorm:"column:unit_sell_price"`
		EffectiveFrom time.Time  `gorm:"column:effective_from"`
		EffectiveTo   *time.Time `gorm:"column:effective_to"`
		ChangedBy     *string    `gorm:"column:changed_by"`
	}
	var rows []row
	err := applyFilters(s.db.WithContext(ctx)).
		Select(`mup.id, mup.product_unit_id, mu.name AS unit_name, mup.unit_sell_price,
		        mup.effective_from, mup.effective_to, mup.changed_by`).
		// mup.id breaks ties: two rows for the same unit can share effective_from
		// (a seed + an immediate edit), and an unstable order would let a row
		// repeat on one page and vanish from the next.
		Order("mu.is_base DESC, mu.factor ASC, mup.effective_from DESC, mup.id DESC").
		Offset(offset).Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.ProductUnitPrice, 0, len(rows))
	for _, r := range rows {
		p := &inventoryifacev1.ProductUnitPrice{
			Id:            r.ID,
			ProductUnitId: r.ProductUnitID,
			UnitName:      r.UnitName,
			UnitSellPrice: r.UnitSellPrice,
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
	return connect.NewResponse(&inventoryifacev1.ListProductUnitPricesResponse{
		Prices: out,
		Total:  int32(total),
	}), nil
}

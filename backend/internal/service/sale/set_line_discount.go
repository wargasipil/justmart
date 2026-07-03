package sale

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// SetLineDiscount sets a per-item discount (FIXED amount or PERCENT) on a cart
// line and recomputes the line + sale totals. The discount_value is minor units
// when FIXED, basis points (percent*100) when PERCENT. DRAFT only; the resolved
// amount lands in line_discount (computeLineTotal re-resolves PERCENT off gross).
func (s *SaleService) SetLineDiscount(
	ctx context.Context,
	req *connect.Request[posifacev1.SetLineDiscountRequest],
) (*connect.Response[posifacev1.SetLineDiscountResponse], error) {
	normType, err := validateDiscountInput(req.Msg.DiscountType, req.Msg.DiscountValue)
	if err != nil {
		return nil, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		var item model.SaleItem
		if err := tx.Where("id = ? AND sale_id = ?", req.Msg.ItemId, sale.ID).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("item not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		// Manual override: the cashier's discount wins over any auto product
		// discount until cleared (ClearLineDiscount reverts to auto).
		item.DiscountManual = true
		item.DiscountType = normType
		item.DiscountValue = req.Msg.DiscountValue
		if err := recomputeLine(tx, &item); err != nil {
			return err
		}
		if err := tx.Save(&item).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return recomputeSaleTotals(tx, sale.ID)
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.SetLineDiscountResponse{Sale: saleToProto(sale)}), nil
}

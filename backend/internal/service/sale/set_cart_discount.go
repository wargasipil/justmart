package sale

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// SetCartDiscount sets a subtotal-level discount (FIXED amount or PERCENT) on the
// sale and recomputes the total. The discount_value is minor units when FIXED,
// basis points (percent*100) when PERCENT. DRAFT only; the resolved amount lands
// in cart_discount (recomputeSaleTotals re-resolves PERCENT off the subtotal).
func (s *SaleService) SetCartDiscount(
	ctx context.Context,
	req *connect.Request[posifacev1.SetCartDiscountRequest],
) (*connect.Response[posifacev1.SetCartDiscountResponse], error) {
	// Validate the type/value up front (surfaces InvalidArgument before the tx).
	normType, err := validateDiscountInput(req.Msg.DiscountType, req.Msg.DiscountValue)
	if err != nil {
		return nil, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		if err := tx.Model(sale).Updates(map[string]any{
			"cart_discount_type":  normType,
			"cart_discount_value": req.Msg.DiscountValue,
		}).Error; err != nil {
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
	return connect.NewResponse(&posifacev1.SetCartDiscountResponse{Sale: saleToProto(sale)}), nil
}

// validateDiscountInput checks a discount type/value pair (without a base) and
// returns the normalized type. Shared by the line + cart discount handlers so a
// bad input is rejected with InvalidArgument before any tx work.
func validateDiscountInput(discType string, value int64) (string, error) {
	// resolveDiscount with a 0 base validates type + value bounds (a 0 base just
	// yields a 0 amount); we only want the normalized type + error here.
	_, normType, err := resolveDiscount(0, discType, value)
	if err != nil {
		return "", err
	}
	return normType, nil
}

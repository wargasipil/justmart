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

func (s *SaleService) SetItemQuantity(
	ctx context.Context,
	req *connect.Request[posifacev1.SetItemQuantityRequest],
) (*connect.Response[posifacev1.SetItemQuantityResponse], error) {
	if req.Msg.Qty <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("qty must be > 0"))
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		factor := item.UnitFactor
		if factor < 1 {
			factor = 1
		}
		item.Qty = req.Msg.Qty
		item.BaseQty = req.Msg.Qty * int32(factor)
		item.LineTotal = computeLineTotal(item.Qty, item.UnitPriceSnapshot, item.LineDiscount)
		if err := tx.Save(&item).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := recomputeSaleTotals(tx, sale.ID); err != nil {
			return err
		}
		// Pharmacy gate: re-check Rx coverage for the new quantity.
		return s.assertRxCovers(ctx, tx, sale, item.ProductID)
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.SetItemQuantityResponse{Sale: saleToProto(sale)}), nil
}

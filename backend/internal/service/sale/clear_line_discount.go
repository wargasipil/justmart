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

// ClearLineDiscount removes a manual line discount and reverts the line to the
// auto-applied product discount (if any). DRAFT only.
func (s *SaleService) ClearLineDiscount(
	ctx context.Context,
	req *connect.Request[posifacev1.ClearLineDiscountRequest],
) (*connect.Response[posifacev1.ClearLineDiscountResponse], error) {
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
		item.DiscountManual = false
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
	return connect.NewResponse(&posifacev1.ClearLineDiscountResponse{Sale: saleToProto(sale)}), nil
}

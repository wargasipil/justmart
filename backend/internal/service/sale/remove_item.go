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

func (s *SaleService) RemoveItem(
	ctx context.Context,
	req *connect.Request[posifacev1.RemoveItemRequest],
) (*connect.Response[posifacev1.RemoveItemResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		res := tx.Where("id = ? AND sale_id = ?", req.Msg.ItemId, sale.ID).Delete(&model.SaleItem{})
		if res.Error != nil {
			return connect.NewError(connect.CodeInternal, res.Error)
		}
		if res.RowsAffected == 0 {
			return connect.NewError(connect.CodeNotFound, errors.New("item not found"))
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
	return connect.NewResponse(&posifacev1.RemoveItemResponse{Sale: saleToProto(sale)}), nil
}

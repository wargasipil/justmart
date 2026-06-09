package sale

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SaleService) VoidSale(
	ctx context.Context,
	req *connect.Request[posifacev1.VoidSaleRequest],
) (*connect.Response[posifacev1.VoidSaleResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sale model.Sale
		if err := common.RowLock(tx).
			Where("id = ?", req.Msg.SaleId).First(&sale).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("sale not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if sale.Status != saleStatusDraft {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("only draft sales can be voided; this one is %s", sale.Status))
		}
		return tx.Model(&sale).Update("status", saleStatusVoided).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.VoidSaleResponse{Sale: saleToProto(sale)}), nil
}

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

// DiscardSale hard-deletes a DRAFT sale + its items so an abandoned cart leaves
// no trace. Safe: a DRAFT has no stock_movements and no sale_no.
func (s *SaleService) DiscardSale(
	ctx context.Context,
	req *connect.Request[posifacev1.DiscardSaleRequest],
) (*connect.Response[posifacev1.DiscardSaleResponse], error) {
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
				fmt.Errorf("only draft sales can be discarded; this one is %s", sale.Status))
		}
		if err := tx.Where("sale_id = ?", sale.ID).Delete(&model.SaleItem{}).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := tx.Delete(&sale).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	return connect.NewResponse(&posifacev1.DiscardSaleResponse{}), nil
}

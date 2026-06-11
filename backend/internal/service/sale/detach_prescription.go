package sale

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SaleService) DetachPrescription(
	ctx context.Context,
	req *connect.Request[posifacev1.DetachPrescriptionRequest],
) (*connect.Response[posifacev1.DetachPrescriptionResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		// Block detach while any Rx-required product is still in the cart — the
		// user must remove those lines first (they'd otherwise be uncovered).
		var rxItemCount int64
		if err := tx.Table("sale_items AS si").
			Joins("JOIN products p ON p.id = si.product_id").
			Where("si.sale_id = ? AND p.prescription_required = ?", sale.ID, true).
			Count(&rxItemCount).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if rxItemCount > 0 {
			return connect.NewError(connect.CodeFailedPrecondition,
				errors.New("remove prescription-required items from the cart before detaching the prescription"))
		}
		return tx.Model(sale).Update("prescription_id", nil).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.DetachPrescriptionResponse{Sale: saleToProto(sale)}), nil
}

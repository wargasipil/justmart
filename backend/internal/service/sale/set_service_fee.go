package sale

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// SetServiceFee overrides the sale's biaya jasa (service fee) at the till. The
// value is snapshotted from the attached resep on AttachPrescription; this lets
// the cashier adjust it. DRAFT only; recomputes the total so the fee is included.
func (s *SaleService) SetServiceFee(
	ctx context.Context,
	req *connect.Request[posifacev1.SetServiceFeeRequest],
) (*connect.Response[posifacev1.SetServiceFeeResponse], error) {
	if req.Msg.BiayaJasa < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("biaya_jasa must be >= 0"))
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		if err := tx.Model(sale).Update("biaya_jasa", req.Msg.BiayaJasa).Error; err != nil {
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
	return connect.NewResponse(&posifacev1.SetServiceFeeResponse{Sale: saleToProto(sale)}), nil
}

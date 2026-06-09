package sale

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SaleService) SetSaleCustomer(
	ctx context.Context,
	req *connect.Request[posifacev1.SetSaleCustomerRequest],
) (*connect.Response[posifacev1.SetSaleCustomerResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		updates := map[string]any{"customer_id": nil}
		if req.Msg.CustomerId != "" {
			updates["customer_id"] = req.Msg.CustomerId
		}
		return tx.Model(sale).Updates(updates).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.SetSaleCustomerResponse{Sale: saleToProto(sale)}), nil
}

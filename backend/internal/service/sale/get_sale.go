package sale

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/auth"
)

func (s *SaleService) GetSale(
	ctx context.Context,
	req *connect.Request[posifacev1.GetSaleRequest],
) (*connect.Response[posifacev1.GetSaleResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	sale, err := s.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	// Cashiers may only open their own sale; managers (OWNER/PHARMACIST) any.
	if (caller.Role == "CASHIER" || caller.Role == "APOTEKER") && sale.CashierUserID != caller.UserID {
		return nil, connect.NewError(connect.CodePermissionDenied,
			errors.New("can only view own sale"))
	}
	return connect.NewResponse(&posifacev1.GetSaleResponse{Sale: saleToProto(sale)}), nil
}

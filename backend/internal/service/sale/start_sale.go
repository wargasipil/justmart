package sale

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SaleService) StartSale(
	ctx context.Context,
	req *connect.Request[posifacev1.StartSaleRequest],
) (*connect.Response[posifacev1.StartSaleResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	// Restaurant counter flows may declare how the order leaves (takeaway /
	// delivery). DINE_IN is rejected here even though it is a valid stored
	// value: seating an order is TableService.OpenTable's job, and that is the
	// only path that binds a table and takes the one-bill-per-table lock — a
	// DINE_IN sale with no table would be a bill nobody can find on the floor.
	orderType := strings.TrimSpace(req.Msg.OrderType)
	if !common.IsCounterOrderType(orderType) {
		return nil, common.TokenError(connect.CodeInvalidArgument, "sale.order_type_invalid")
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	sale := model.Sale{
		CashierUserID: caller.UserID,
		Status:        saleStatusDraft,
		WarehouseID:   &warehouseID,
		OrderType:     orderType,
	}
	if err := s.db.WithContext(ctx).Create(&sale).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out, err := s.loadFull(ctx, sale.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.StartSaleResponse{Sale: saleToProto(out)}), nil
}

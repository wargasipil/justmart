package table

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	tableifacev1 "github.com/justmart/backend/gen/table_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// MoveOrder re-seats an open bill onto a different table — a party changing
// tables, or a takeaway order that decides to sit down (moving a counter order
// onto a table also promotes it to DINE_IN, which is the honest description of
// what just happened).
//
// The destination must be free: merging two bills is a different operation with
// its own questions (whose discounts survive? which fired tickets?) and is
// deliberately out of scope rather than silently implied by a move.
func (s *TableService) MoveOrder(
	ctx context.Context,
	req *connect.Request[tableifacev1.MoveOrderRequest],
) (*connect.Response[tableifacev1.MoveOrderResponse], error) {
	dest, err := s.load(ctx, req.Msg.ToTableId)
	if err != nil {
		return nil, err
	}
	if !dest.Active {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "table.archived")
	}

	var sale model.Sale
	err = s.db.WithContext(ctx).Where("id = ?", req.Msg.SaleId).First(&sale).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("sale %s not found", req.Msg.SaleId))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// Only an OPEN bill can move. A settled one is history.
	if sale.Status != common.SaleStatusDraft {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "table.sale_not_open")
	}
	// Tables live in an outlet and stock is consumed from that outlet, so a bill
	// can't hop between outlets by way of a table move.
	if sale.WarehouseID == nil || *sale.WarehouseID != dest.WarehouseID {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "table.other_warehouse")
	}
	if sale.TableID != nil && *sale.TableID == dest.ID {
		// Already there — nothing to do, and reporting a conflict against itself
		// would be nonsense.
		return s.moved(ctx, dest)
	}
	if _, occupied, err := s.openSaleID(ctx, dest.ID); err != nil {
		return nil, err
	} else if occupied {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "table.occupied")
	}

	if err := s.db.WithContext(ctx).Model(&model.Sale{}).
		Where("id = ?", sale.ID).
		Updates(map[string]any{
			"table_id":   dest.ID,
			"order_type": common.OrderTypeDineIn,
		}).Error; err != nil {
		// The partial unique index backstops a concurrent open on the destination.
		return nil, common.TokenError(connect.CodeFailedPrecondition, "table.occupied")
	}
	return s.moved(ctx, dest)
}

func (s *TableService) moved(
	ctx context.Context,
	dest *model.DiningTable,
) (*connect.Response[tableifacev1.MoveOrderResponse], error) {
	out := toProto(dest)
	bills, err := loadOpenBills(ctx, s.db, []string{dest.ID})
	if err != nil {
		return nil, err
	}
	applyOccupancy(out, bills)
	return connect.NewResponse(&tableifacev1.MoveOrderResponse{Table: out}), nil
}

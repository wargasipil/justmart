package table

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	tableifacev1 "github.com/justmart/backend/gen/table_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// OpenTable seats an order at a table: it returns the table's existing open bill
// if there is one, otherwise it starts a fresh DRAFT sale bound to the table.
//
// Tapping an occupied table is the NORMAL way to resume serving it, not an
// error — a waiter adding a second round taps the same tile they opened. The
// response's `resumed` flag lets the UI say which happened without the caller
// having to check first.
//
// Concurrency: two waiters tapping the same free table at the same moment is an
// ordinary rush-hour event, not a rare race. Rather than lock, this leans on the
// partial unique index (one DRAFT per table) — the loser's INSERT fails, and it
// re-reads and returns the winner's bill as a resume. Both waiters end up on the
// same order, which is exactly right.
func (s *TableService) OpenTable(
	ctx context.Context,
	req *connect.Request[tableifacev1.OpenTableRequest],
) (*connect.Response[tableifacev1.OpenTableResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	t, err := s.load(ctx, req.Msg.TableId)
	if err != nil {
		return nil, err
	}
	if !t.Active {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "table.archived")
	}
	if req.Msg.GuestCount < 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "table.guest_count_invalid")
	}

	db := s.db.WithContext(ctx)

	if existing, ok, err := s.openSaleID(ctx, t.ID); err != nil {
		return nil, err
	} else if ok {
		return s.respond(ctx, t, existing, true)
	}

	sale := &model.Sale{
		CashierUserID: caller.UserID,
		Status:        common.SaleStatusDraft,
		// The bill belongs to the TABLE's outlet, not the caller's header: a
		// manager viewing another outlet must not open a bill that then consumes
		// stock from the wrong warehouse at completion.
		WarehouseID: &t.WarehouseID,
		TableID:     &t.ID,
		OrderType:   common.OrderTypeDineIn,
		GuestCount:  req.Msg.GuestCount,
	}
	if err := db.Create(sale).Error; err != nil {
		// Lost the race — the index rejected the second DRAFT. Return the winner's.
		if existing, ok, lookupErr := s.openSaleID(ctx, t.ID); lookupErr == nil && ok {
			return s.respond(ctx, t, existing, true)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return s.respond(ctx, t, sale.ID, false)
}

// openSaleID returns the id of the table's current DRAFT sale, if any.
func (s *TableService) openSaleID(ctx context.Context, tableID string) (string, bool, error) {
	var sale model.Sale
	err := s.db.WithContext(ctx).
		Select("id").
		Where("table_id = ? AND status = ?", tableID, common.SaleStatusDraft).
		First(&sale).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, connect.NewError(connect.CodeInternal, err)
	}
	return sale.ID, true, nil
}

func (s *TableService) respond(
	ctx context.Context,
	t *model.DiningTable,
	saleID string,
	resumed bool,
) (*connect.Response[tableifacev1.OpenTableResponse], error) {
	out := toProto(t)
	bills, err := loadOpenBills(ctx, s.db, []string{t.ID})
	if err != nil {
		return nil, err
	}
	applyOccupancy(out, bills)
	return connect.NewResponse(&tableifacev1.OpenTableResponse{
		SaleId:  saleID,
		Resumed: resumed,
		Table:   out,
	}), nil
}

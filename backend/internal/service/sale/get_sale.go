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
	// Till/floor roles may only open their own sale; managers (OWNER/PHARMACIST)
	// any.
	//
	// A TABLE-BOUND bill is the exception: a dining table is shared by the floor,
	// so whoever is nearest picks it up. Self-scoping an open bill to the waiter
	// who happened to seat the party would mean nobody else could add a round or
	// take it to the till — the opposite of how a restaurant runs. (A settled
	// dine-in order is history and self-scopes again like any other sale.)
	sharedOpenBill := sale.TableID != nil && *sale.TableID != "" && sale.Status == saleStatusDraft
	switch caller.Role {
	case "CASHIER", "APOTEKER", "WAITER":
		if !sharedOpenBill && sale.CashierUserID != caller.UserID {
			return nil, connect.NewError(connect.CodePermissionDenied,
				errors.New("can only view own sale"))
		}
	}

	out := saleToProto(sale)
	// The floor label is what POS shows in the cart header ("Meja T4"), so it
	// travels with the bill rather than needing a second lookup.
	if err := s.enrichTableCodes(ctx, []*posifacev1.Sale{out}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.GetSaleResponse{Sale: out}), nil
}

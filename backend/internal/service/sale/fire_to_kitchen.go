package sale

import (
	"context"
	"time"

	"connectrpc.com/connect"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/printer"
	"github.com/justmart/backend/internal/service/common"
)

// FireToKitchen sends the lines added since the last fire to the kitchen printer
// and stamps them fired.
//
// INCREMENTAL BY DESIGN. A dine-in bill grows across rounds, so a fire must
// carry only what is new — reprinting the whole order every round would have the
// kitchen cook the starters twice. That is why fired_at lives on the LINE.
//
// Firing with nothing new is a no-op success (fired_items = 0), not an error:
// a waiter double-tapping Fire should not be told off, and it must not reprint.
//
// The stamp is written in the SAME transaction that renders, and only after the
// payload actually reaches a printer — a failed print leaves the lines unfired
// so the next tap retries them. The reverse (stamp first, print after) would
// silently lose an order on any printer hiccup, which in a kitchen means food
// that never gets cooked.
func (s *SaleService) FireToKitchen(
	ctx context.Context,
	req *connect.Request[posifacev1.FireToKitchenRequest],
) (*connect.Response[posifacev1.FireToKitchenResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.assertPrintingConfigured(); err != nil {
		return nil, err
	}

	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	if sale.Status != saleStatusDraft {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "sale.not_open")
	}

	// Unfired lines, oldest first — the order they were called in.
	var pending []model.SaleItem
	if err := s.db.WithContext(ctx).
		Where("sale_id = ? AND fired_at IS NULL", sale.ID).
		Order("created_at ASC, id ASC").
		Find(&pending).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	round, err := s.fireRound(ctx, sale.ID)
	if err != nil {
		return nil, err
	}
	if len(pending) == 0 {
		return connect.NewResponse(&posifacev1.FireToKitchenResponse{
			FiredItems: 0,
			Round:      int32(round),
		}), nil
	}

	ticket, err := s.buildKitchenTicket(ctx, caller, sale, pending, round+1)
	if err != nil {
		return nil, err
	}
	payload := printer.RenderKitchenTicket(ticket, s.printSettings(ctx))

	target := printTarget{
		DeviceID:    req.Msg.ConnectorDeviceId,
		PrinterName: req.Msg.PrinterName,
	}
	if err := s.dispatch(ctx, target, s.kitchenTarget, payload, "kitchen-"+sale.ID); err != nil {
		return nil, err
	}

	// Printed — now stamp, so a printer failure above leaves the lines to retry.
	now := time.Now()
	ids := make([]string, len(pending))
	for i := range pending {
		ids[i] = pending[i].ID
	}
	if err := s.db.WithContext(ctx).Model(&model.SaleItem{}).
		Where("id IN ?", ids).
		Update("fired_at", now).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&posifacev1.FireToKitchenResponse{
		FiredItems: int32(len(pending)),
		Round:      int32(round + 1),
		BytesSent:  int32(len(payload)),
	}), nil
}

// fireRound counts how many fires this bill has already had, by counting the
// DISTINCT fired_at stamps on its lines. Every line fired together shares one
// timestamp, so distinct stamps == rounds — no counter column needed, and it
// stays correct even if lines are removed afterwards.
func (s *SaleService) fireRound(ctx context.Context, saleID string) (int, error) {
	var n int64
	if err := s.db.WithContext(ctx).
		Model(&model.SaleItem{}).
		Where("sale_id = ? AND fired_at IS NOT NULL", saleID).
		Distinct("fired_at").
		Count(&n).Error; err != nil {
		return 0, connect.NewError(connect.CodeInternal, err)
	}
	return int(n), nil
}

// buildKitchenTicket denormalizes everything the printer package needs (it never
// touches the DB): product names, the floor label, and who fired it.
func (s *SaleService) buildKitchenTicket(
	ctx context.Context,
	caller auth.Principal,
	sale *model.Sale,
	lines []model.SaleItem,
	round int,
) (printer.KitchenTicket, error) {
	productIDs := make([]string, 0, len(lines))
	for i := range lines {
		productIDs = append(productIDs, lines[i].ProductID)
	}
	var prods []model.Product
	if err := s.db.WithContext(ctx).
		Select("id", "name").Where("id IN ?", productIDs).Find(&prods).Error; err != nil {
		return printer.KitchenTicket{}, connect.NewError(connect.CodeInternal, err)
	}
	nameByID := make(map[string]string, len(prods))
	for i := range prods {
		nameByID[prods[i].ID] = prods[i].Name
	}

	tableCode := ""
	if sale.TableID != nil && *sale.TableID != "" {
		var tbl model.DiningTable
		if err := s.db.WithContext(ctx).
			Select("code").Where("id = ?", *sale.TableID).First(&tbl).Error; err == nil {
			tableCode = tbl.Code
		}
	}

	server := ""
	var u model.User
	if err := s.db.WithContext(ctx).
		Select("name", "email").Where("id = ?", caller.UserID).First(&u).Error; err == nil {
		server = u.Name
		if server == "" {
			server = u.Email
		}
	}

	items := make([]printer.KitchenLine, 0, len(lines))
	for i := range lines {
		items = append(items, printer.KitchenLine{
			Qty:      lines[i].Qty,
			UnitName: lines[i].UnitName,
			Name:     nameByID[lines[i].ProductID],
			Note:     lines[i].KitchenNote,
		})
	}
	return printer.KitchenTicket{
		TableCode:  tableCode,
		OrderType:  sale.OrderType,
		GuestCount: sale.GuestCount,
		Server:     server,
		FiredAt:    time.Now(),
		Round:      round,
		Items:      items,
	}, nil
}

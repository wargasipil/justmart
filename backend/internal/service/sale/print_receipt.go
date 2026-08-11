package sale

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/printer"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SaleService) PrintReceipt(
	ctx context.Context,
	req *connect.Request[posifacev1.PrintReceiptRequest],
) (*connect.Response[posifacev1.PrintReceiptResponse], error) {
	if err := s.print.EnsureConfigured(); err != nil {
		return nil, err
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	if sale.Status != saleStatusCompleted {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("only completed sales can be printed"))
	}

	// Resolve product names.
	medIDs := make([]string, 0, len(sale.Items))
	for _, it := range sale.Items {
		medIDs = append(medIDs, it.ProductID)
	}
	var meds []model.Product
	if err := s.db.WithContext(ctx).Where("id IN ?", medIDs).Find(&meds).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	medName := make(map[string]string, len(meds))
	for _, m := range meds {
		medName[m.ID] = m.Name
	}

	// Resolve cashier name.
	var cashier model.User
	cashierName := ""
	if err := s.db.WithContext(ctx).Where("id = ?", sale.CashierUserID).First(&cashier).Error; err == nil {
		cashierName = cashier.Name
		if cashierName == "" {
			cashierName = cashier.Email
		}
	}

	// Resolve customer name (optional).
	customerName := ""
	if sale.CustomerID != nil && *sale.CustomerID != "" {
		var customer model.Customer
		if err := s.db.WithContext(ctx).Where("id = ?", *sale.CustomerID).First(&customer).Error; err == nil {
			customerName = customer.Name
		}
	}

	// Build the renderer Receipt.
	lines := make([]printer.ReceiptLine, 0, len(sale.Items))
	for _, it := range sale.Items {
		lines = append(lines, printer.ReceiptLine{
			Qty:       it.Qty,
			UnitName:  it.UnitName,
			Name:      medName[it.ProductID],
			LineTotal: it.LineTotal,
		})
	}
	completedAt := time.Time{}
	if sale.CompletedAt != nil {
		completedAt = *sale.CompletedAt
	}
	saleNo := ""
	if sale.SaleNo != nil {
		saleNo = *sale.SaleNo
	}
	paymentStr := ""
	if sale.PaymentSource != nil {
		paymentStr = *sale.PaymentSource
	}
	change := int64(0)
	if paymentStr == paymentCash && sale.PaidAmount > sale.Total {
		change = sale.PaidAmount - sale.Total
	}

	receipt := printer.Receipt{
		SaleNo:       saleNo,
		CompletedAt:  completedAt,
		Cashier:      cashierName,
		Customer:     customerName,
		Items:        lines,
		Subtotal:     sale.Subtotal,
		CartDiscount: sale.CartDiscount,
		BiayaJasa:    sale.BiayaJasa,
		Total:        sale.Total,
		Paid:         sale.PaidAmount,
		Payment:      paymentStr,
		Change:       change,
	}
	// Header/footer/width come from app_settings (seeded at boot from config.yaml,
	// editable in Settings ▸ Printing); fall back to the config only if the
	// lookup errors. Drawer stays hardware config.
	header, footer := s.printer.Header, s.printer.Footer
	if h, f, err := common.GetReceiptText(ctx, s.db); err == nil {
		header = common.ReceiptLines(h)
		footer = common.ReceiptLines(f)
	}
	width := s.printer.Width
	if w, err := common.GetReceiptWidth(ctx, s.db); err == nil {
		width = int(w)
	}
	settings := printer.Settings{
		Width:      width,
		Header:     header,
		Footer:     footer,
		OpenDrawer: s.printer.OpenDrawer,
	}
	payload := printer.Render(receipt, settings)

	if err := s.print.Dispatch(
		ctx, s.db,
		req.Msg.ConnectorDeviceId, req.Msg.PrinterName,
		payload, req.Msg.SaleId,
	); err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.PrintReceiptResponse{
		BytesSent: int32(len(payload)),
	}), nil
}

package payroll

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/printer"
	"github.com/justmart/backend/internal/service/common"
)

var indoMonths = []string{
	"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

func indoMonth(m int32) string {
	if m >= 1 && m <= 12 {
		return indoMonths[m]
	}
	return ""
}

// bankLabel builds a one-line bank string from the payslip's snapshot, e.g.
// "BCA 12345 (a.n. Budi)". Empty when no bank info.
func bankLabel(p *model.Payslip) string {
	parts := []string{}
	if p.BankName != "" {
		parts = append(parts, p.BankName)
	}
	if p.BankAccountNumber != "" {
		parts = append(parts, p.BankAccountNumber)
	}
	label := strings.Join(parts, " ")
	if p.BankAccountHolder != "" {
		if label != "" {
			label += " "
		}
		label += "(a.n. " + p.BankAccountHolder + ")"
	}
	return label
}

// PrintPayslip renders a payslip as ESC/POS and dispatches it. Mirrors
// SaleService.PrintReceipt's mode switch (connector / usb / tcp).
func (s *PayrollService) PrintPayslip(
	ctx context.Context,
	req *connect.Request[payrollifacev1.PrintPayslipRequest],
) (*connect.Response[payrollifacev1.PrintPayslipResponse], error) {
	mode := s.connectorCfg.Mode
	if mode != "connector" && mode != "usb" && !s.printer.Enabled {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("printing is not configured (set printer.enabled, or connector.mode to connector/usb, in config.yaml)"))
	}
	if req.Msg.PayslipId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("payslip_id required"))
	}

	var slip model.Payslip
	if err := s.db.WithContext(ctx).
		Preload("Components", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order") }).
		Where("id = ?", req.Msg.PayslipId).First(&slip).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("payslip not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var run model.PayrollRun
	if err := s.db.WithContext(ctx).Where("id = ?", slip.RunID).First(&run).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	view := printer.Payslip{
		RunNo:           common.Deref(run.RunNo),
		PeriodLabel:     fmt.Sprintf("%s %d", indoMonth(run.PeriodMonth), run.PeriodYear),
		EmployeeName:    slip.EmployeeName,
		EmployeeCode:    slip.EmployeeCode,
		Position:        slip.Position,
		Bank:            bankLabel(&slip),
		Gross:           slip.Gross,
		TotalDeductions: slip.TotalDeductions,
		Net:             slip.Net,
		GeneratedAt:     time.Now(),
	}
	for i := range slip.Components {
		c := &slip.Components[i]
		switch c.Kind {
		case kindEarning:
			view.Earnings = append(view.Earnings, printer.PayslipLine{Label: c.Label, Amount: c.Amount})
		case kindDeduction:
			view.Deductions = append(view.Deductions, printer.PayslipLine{Label: c.Label, Amount: c.Amount})
		}
	}

	header, footer := s.printer.Header, s.printer.Footer
	if h, f, err := common.GetReceiptText(ctx, s.db); err == nil {
		header = common.ReceiptLines(h)
		footer = common.ReceiptLines(f)
	}
	width := s.printer.Width
	if w, err := common.GetReceiptWidth(ctx, s.db); err == nil {
		width = int(w)
	}
	payload := printer.RenderPayslip(view, printer.Settings{Width: width, Header: header, Footer: footer})

	switch mode {
	case "connector":
		if s.connector == nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("connector mode is on but no print connector registry is wired"))
		}
		deviceID := req.Msg.ConnectorDeviceId
		printerName := req.Msg.PrinterName
		if deviceID == "" {
			d, p, derr := common.GetPrintTarget(ctx, s.db)
			if derr != nil {
				return nil, connect.NewError(connect.CodeInternal, derr)
			}
			deviceID = d
			if printerName == "" {
				printerName = p
			}
		}
		if _, err := s.connector.Push(deviceID, printerName, payload); err != nil {
			return nil, err
		}
	case "usb":
		printerName := req.Msg.PrinterName
		if printerName == "" {
			_, saved, derr := common.GetPrintTarget(ctx, s.db)
			if derr != nil {
				return nil, connect.NewError(connect.CodeInternal, derr)
			}
			printerName = saved
		}
		if printerName == "" {
			printerName = s.connectorCfg.PrinterName
		}
		if err := s.spool(printerName, payload, req.Msg.PayslipId); err != nil {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
	default: // "tcp"
		if err := printer.DispatchTCP(s.printer.Address, payload, s.printer.Timeout); err != nil {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
	}
	return connect.NewResponse(&payrollifacev1.PrintPayslipResponse{
		BytesSent: int32(len(payload)),
	}), nil
}

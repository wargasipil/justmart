package payroll

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/service/payment"
)

// MarkPayrollRunPaid moves a run APPROVED -> PAID. Two methods:
//   - MANUAL (default): just records the PAID transition (no money movement).
//   - GATEWAY: also disburses each payslip's net pay to the employee's bank /
//     e-wallet via the payment-integration layer (the active provider).
func (s *PayrollService) MarkPayrollRunPaid(
	ctx context.Context,
	req *connect.Request[payrollifacev1.MarkPayrollRunPaidRequest],
) (*connect.Response[payrollifacev1.MarkPayrollRunPaidResponse], error) {
	method := strings.ToUpper(strings.TrimSpace(req.Msg.Method))

	if method == "" || method == "MANUAL" {
		if err := s.transition(ctx, req.Msg.Id, []string{runStatusApproved}, runStatusPaid, "paid_at"); err != nil {
			return nil, common.AsConnectErr(err)
		}
		run, err := s.runResponse(ctx, req.Msg.Id)
		if err != nil {
			return nil, err
		}
		return connect.NewResponse(&payrollifacev1.MarkPayrollRunPaidResponse{Run: run}), nil
	}

	if method != "GATEWAY" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.pay_method_invalid")
	}
	return s.markPaidViaGateway(ctx, req.Msg.Id)
}

// markPaidViaGateway validates payout details, stamps the run PAID, then submits
// one disbursement per payslip. The provider HTTP calls happen OUTSIDE the PAID
// transaction (CreateDisbursement holds no DB tx across the network). A payslip
// whose disbursement fails is retryable individually; the run stays PAID.
func (s *PayrollService) markPaidViaGateway(ctx context.Context, runID string) (*connect.Response[payrollifacev1.MarkPayrollRunPaidResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if s.disburser == nil {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "payment.no_provider")
	}

	// Load the run + payslips and validate payout details BEFORE any money moves.
	run, err := s.loadRunFull(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.Status != runStatusApproved {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "payroll.invalid_transition")
	}
	for i := range run.Payslips {
		p := &run.Payslips[i]
		if strings.TrimSpace(p.BankChannelCode) == "" ||
			strings.TrimSpace(p.BankAccountNumber) == "" ||
			p.Net <= 0 {
			return nil, common.TokenError(connect.CodeFailedPrecondition, "payroll.missing_channel_code")
		}
	}

	// Stamp PAID (own tx, no provider call inside).
	if err := s.transition(ctx, runID, []string{runStatusApproved}, runStatusPaid, "paid_at"); err != nil {
		return nil, common.AsConnectErr(err)
	}

	// Submit one disbursement per payslip (each: insert PENDING + submit + update,
	// no shared tx). Best-effort per payslip — a failure becomes a FAILED row that
	// the OWNER can retry.
	for i := range run.Payslips {
		p := &run.Payslips[i]
		_, _ = s.disburser.CreateDisbursement(ctx, payment.DisburseRequest{
			ReferenceType: "payslip",
			ReferenceID:   p.ID,
			ChannelCode:   p.BankChannelCode,
			AccountNumber: p.BankAccountNumber,
			AccountHolder: payslipHolder(p),
			Amount:        p.Net,
			Currency:      "IDR",
			CreatedBy:     caller.UserID,
		})
	}

	out, err := s.runResponse(ctx, runID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&payrollifacev1.MarkPayrollRunPaidResponse{Run: out}), nil
}

// payslipHolder falls back to the employee name when no explicit account holder
// was recorded.
func payslipHolder(p *model.Payslip) string {
	if strings.TrimSpace(p.BankAccountHolder) != "" {
		return p.BankAccountHolder
	}
	return p.EmployeeName
}

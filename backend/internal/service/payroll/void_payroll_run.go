package payroll

import (
	"context"

	"connectrpc.com/connect"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// VoidPayrollRun voids a DRAFT or APPROVED run (a PAID run cannot be voided). A
// voided run frees the one-live-run-per-month slot for a redo.
func (s *PayrollService) VoidPayrollRun(
	ctx context.Context,
	req *connect.Request[payrollifacev1.VoidPayrollRunRequest],
) (*connect.Response[payrollifacev1.VoidPayrollRunResponse], error) {
	if err := s.transition(ctx, req.Msg.Id, []string{runStatusDraft, runStatusApproved}, runStatusVoided, "voided_at"); err != nil {
		return nil, common.AsConnectErr(err)
	}
	run, err := s.runResponse(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&payrollifacev1.VoidPayrollRunResponse{Run: run}), nil
}

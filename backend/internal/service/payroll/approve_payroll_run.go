package payroll

import (
	"context"

	"connectrpc.com/connect"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// ApprovePayrollRun moves a run DRAFT -> APPROVED, freezing component edits.
func (s *PayrollService) ApprovePayrollRun(
	ctx context.Context,
	req *connect.Request[payrollifacev1.ApprovePayrollRunRequest],
) (*connect.Response[payrollifacev1.ApprovePayrollRunResponse], error) {
	if err := s.transition(ctx, req.Msg.Id, []string{runStatusDraft}, runStatusApproved, "approved_at"); err != nil {
		return nil, common.AsConnectErr(err)
	}
	run, err := s.runResponse(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&payrollifacev1.ApprovePayrollRunResponse{Run: run}), nil
}

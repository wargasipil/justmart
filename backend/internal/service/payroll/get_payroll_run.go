package payroll

import (
	"context"

	"connectrpc.com/connect"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
)

func (s *PayrollService) GetPayrollRun(
	ctx context.Context,
	req *connect.Request[payrollifacev1.GetPayrollRunRequest],
) (*connect.Response[payrollifacev1.GetPayrollRunResponse], error) {
	run, err := s.loadRunFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	out := runToProto(run, true)
	s.enrichDisbursements(ctx, out)
	return connect.NewResponse(&payrollifacev1.GetPayrollRunResponse{Run: out}), nil
}

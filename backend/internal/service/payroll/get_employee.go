package payroll

import (
	"context"

	"connectrpc.com/connect"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
)

func (s *EmployeeService) GetEmployee(
	ctx context.Context,
	req *connect.Request[payrollifacev1.GetEmployeeRequest],
) (*connect.Response[payrollifacev1.GetEmployeeResponse], error) {
	emp, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&payrollifacev1.GetEmployeeResponse{Employee: employeeToProto(emp)}), nil
}

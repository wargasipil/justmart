package payroll

import (
	"context"

	"connectrpc.com/connect"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
)

func (s *EmployeeService) ArchiveEmployee(
	ctx context.Context,
	req *connect.Request[payrollifacev1.ArchiveEmployeeRequest],
) (*connect.Response[payrollifacev1.ArchiveEmployeeResponse], error) {
	emp, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(emp).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	emp.Active = false
	return connect.NewResponse(&payrollifacev1.ArchiveEmployeeResponse{Employee: employeeToProto(emp)}), nil
}

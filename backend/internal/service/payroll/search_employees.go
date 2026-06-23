package payroll

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *EmployeeService) SearchEmployees(
	ctx context.Context,
	req *connect.Request[payrollifacev1.SearchEmployeesRequest],
) (*connect.Response[payrollifacev1.SearchEmployeesResponse], error) {
	query := strings.TrimSpace(req.Msg.Query)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := s.db.WithContext(ctx).Where("active = ?", true).Order("name").Limit(limit)
	if query != "" {
		pattern := "%" + query + "%"
		op := common.LikeOp(q)
		q = q.Where("name "+op+" ? OR code "+op+" ?", pattern, pattern)
	}
	var rows []model.Employee
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*payrollifacev1.Employee, 0, len(rows))
	for i := range rows {
		out = append(out, employeeToProto(&rows[i]))
	}
	return connect.NewResponse(&payrollifacev1.SearchEmployeesResponse{Employees: out}), nil
}

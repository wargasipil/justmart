package payroll

import (
	"context"

	"connectrpc.com/connect"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ResolveEmployees returns minimal display refs for a set of ids (batch
// lookup-by-IDs for name resolution). Unknown ids are omitted.
func (s *EmployeeService) ResolveEmployees(
	ctx context.Context,
	req *connect.Request[payrollifacev1.ResolveEmployeesRequest],
) (*connect.Response[payrollifacev1.ResolveEmployeesResponse], error) {
	ids := common.DedupeIDs(req.Msg.Ids)
	if len(ids) == 0 {
		return connect.NewResponse(&payrollifacev1.ResolveEmployeesResponse{}), nil
	}
	type row struct {
		ID   string `gorm:"column:id"`
		Code string `gorm:"column:code"`
		Name string `gorm:"column:name"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Model(&model.Employee{}).
		Select("id, code, name").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*payrollifacev1.EmployeeRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &payrollifacev1.EmployeeRef{Id: r.ID, Code: r.Code, Name: r.Name})
	}
	return connect.NewResponse(&payrollifacev1.ResolveEmployeesResponse{Employees: out}), nil
}

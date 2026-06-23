package payroll

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *EmployeeService) ListEmployees(
	ctx context.Context,
	req *connect.Request[payrollifacev1.ListEmployeesRequest],
) (*connect.Response[payrollifacev1.ListEmployeesResponse], error) {
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	query := strings.TrimSpace(req.Msg.Query)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		if !req.Msg.IncludeInactive {
			q = q.Where("active = ?", true)
		}
		if query != "" {
			pattern := "%" + query + "%"
			q = q.Where("name "+common.LikeOp(q)+" ? OR code "+common.LikeOp(q)+" ?", pattern, pattern)
		}
		return q
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Employee{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.Employee
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Employee{})).
		Preload("Allowances", func(db *gorm.DB) *gorm.DB { return db.Order("created_at") }).
		Order("name").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*payrollifacev1.Employee, 0, len(rows))
	for i := range rows {
		out = append(out, employeeToProto(&rows[i]))
	}
	return connect.NewResponse(&payrollifacev1.ListEmployeesResponse{
		Employees: out,
		Total:     int32(total),
	}), nil
}

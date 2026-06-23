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

func (s *PayrollService) ListPayrollRuns(
	ctx context.Context,
	req *connect.Request[payrollifacev1.ListPayrollRunsRequest],
) (*connect.Response[payrollifacev1.ListPayrollRunsResponse], error) {
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	status := strings.ToUpper(strings.TrimSpace(req.Msg.Status))
	applyFilters := func(q *gorm.DB) *gorm.DB {
		if status != "" {
			q = q.Where("status = ?", status)
		}
		if req.Msg.Year > 0 {
			q = q.Where("period_year = ?", req.Msg.Year)
		}
		return q
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.PayrollRun{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.PayrollRun
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.PayrollRun{})).
		Order("period_year DESC, period_month DESC, created_at DESC").
		Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*payrollifacev1.PayrollRun, 0, len(rows))
	for i := range rows {
		out = append(out, runToProto(&rows[i], false))
	}
	return connect.NewResponse(&payrollifacev1.ListPayrollRunsResponse{
		Runs:  out,
		Total: int32(total),
	}), nil
}

package payroll

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// RemovePayslipComponent deletes a component from a payslip (DRAFT only) and
// recomputes the payslip + run rollups.
func (s *PayrollService) RemovePayslipComponent(
	ctx context.Context,
	req *connect.Request[payrollifacev1.RemovePayslipComponentRequest],
) (*connect.Response[payrollifacev1.RemovePayslipComponentResponse], error) {
	var runID string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		comp, slip, run, err := loadComponentRun(tx, req.Msg.ComponentId)
		if err != nil {
			return err
		}
		if run.Status != runStatusDraft {
			return common.TokenError(connect.CodeFailedPrecondition, "payroll.run_not_editable")
		}
		runID = run.ID
		if err := tx.Where("id = ?", comp.ID).Delete(&model.PayslipComponent{}).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := recomputePayslip(tx, slip.ID); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return recomputeRun(tx, run.ID)
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	run, err := s.runResponse(ctx, runID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&payrollifacev1.RemovePayslipComponentResponse{Run: run}), nil
}

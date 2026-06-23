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

// UpdatePayslipComponent edits one component's label/amount/taxable (DRAFT only).
// Editing a system-generated line flags it overridden so the audit shows the
// OWNER changed the computed amount.
func (s *PayrollService) UpdatePayslipComponent(
	ctx context.Context,
	req *connect.Request[payrollifacev1.UpdatePayslipComponentRequest],
) (*connect.Response[payrollifacev1.UpdatePayslipComponentResponse], error) {
	if req.Msg.Amount < 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.amount_negative")
	}
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
		label := strings.TrimSpace(req.Msg.Label)
		if label == "" {
			label = comp.Label
		}
		updates := map[string]any{
			"label":   label,
			"amount":  req.Msg.Amount,
			"taxable": req.Msg.Taxable,
		}
		if comp.SystemGenerated {
			updates["overridden"] = true
		}
		if err := tx.Model(&model.PayslipComponent{}).Where("id = ?", comp.ID).Updates(updates).Error; err != nil {
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
	return connect.NewResponse(&payrollifacev1.UpdatePayslipComponentResponse{Run: run}), nil
}

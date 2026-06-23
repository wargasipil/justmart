package payroll

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// AddPayslipComponent appends a manual EARNING (bonus) or DEDUCTION (manual)
// line to a payslip (DRAFT only).
func (s *PayrollService) AddPayslipComponent(
	ctx context.Context,
	req *connect.Request[payrollifacev1.AddPayslipComponentRequest],
) (*connect.Response[payrollifacev1.AddPayslipComponentResponse], error) {
	kind := strings.ToUpper(strings.TrimSpace(req.Msg.Kind))
	if kind != kindEarning && kind != kindDeduction {
		return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.kind_invalid")
	}
	code := strings.ToUpper(strings.TrimSpace(req.Msg.Code))
	switch code {
	case codeBonus, codeManual:
		// allowed as-is
	default:
		// default the code from the kind
		if kind == kindEarning {
			code = codeBonus
		} else {
			code = codeManual
		}
	}
	label := strings.TrimSpace(req.Msg.Label)
	if label == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.label_required")
	}
	if req.Msg.Amount < 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.amount_negative")
	}

	var runID string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if req.Msg.PayslipId == "" {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("payslip_id required"))
		}
		var slip model.Payslip
		if err := tx.Where("id = ?", req.Msg.PayslipId).First(&slip).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("payslip not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		var run model.PayrollRun
		if err := tx.Where("id = ?", slip.RunID).First(&run).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if run.Status != runStatusDraft {
			return common.TokenError(connect.CodeFailedPrecondition, "payroll.run_not_editable")
		}
		runID = run.ID

		var maxSort *int32
		if err := tx.Model(&model.PayslipComponent{}).
			Where("payslip_id = ?", slip.ID).
			Select("MAX(sort_order)").Scan(&maxSort).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		next := int32(0)
		if maxSort != nil {
			next = *maxSort + 1
		}
		comp := model.PayslipComponent{
			PayslipID:       slip.ID,
			Kind:            kind,
			Code:            code,
			Label:           label,
			Amount:          req.Msg.Amount,
			Taxable:         kind == kindEarning && req.Msg.Taxable,
			SystemGenerated: false,
			SortOrder:       next,
		}
		if err := tx.Create(&comp).Error; err != nil {
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
	return connect.NewResponse(&payrollifacev1.AddPayslipComponentResponse{Run: run}), nil
}

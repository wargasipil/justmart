package payroll

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// assignPayrollNo increments the per-year counter and returns PAY-YYYY-NNNN.
// Mirrors assignRxNo / assignSaleNo (atomic UPDATE; no explicit row lock needed).
func assignPayrollNo(tx *gorm.DB, now time.Time) (string, error) {
	year := now.Year()
	var counter model.PayrollNoCounter
	err := tx.Where("year = ?", year).First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		counter = model.PayrollNoCounter{Year: year, LastSeq: 0}
		if err := tx.Create(&counter).Error; err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	if err := tx.Model(&model.PayrollNoCounter{}).
		Where("year = ?", year).
		Update("last_seq", gorm.Expr("last_seq + 1")).Error; err != nil {
		return "", err
	}
	if err := tx.Where("year = ?", year).First(&counter).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("PAY-%d-%04d", year, counter.LastSeq), nil
}

// loadRunFull reads a run with its payslips + components preloaded, ordered for
// stable rendering.
func (s *PayrollService) loadRunFull(ctx context.Context, id string) (*model.PayrollRun, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var run model.PayrollRun
	err := s.db.WithContext(ctx).
		Preload("Payslips", func(db *gorm.DB) *gorm.DB { return db.Order("employee_name") }).
		Preload("Payslips.Components", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order") }).
		Where("id = ?", id).First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("payroll run %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &run, nil
}

// runResponse loads the full run and wraps it for a mutation response.
func (s *PayrollService) runResponse(ctx context.Context, runID string) (*payrollifacev1.PayrollRun, error) {
	full, err := s.loadRunFull(ctx, runID)
	if err != nil {
		return nil, err
	}
	out := runToProto(full, true)
	s.enrichDisbursements(ctx, out)
	return out, nil
}

// enrichDisbursements fills each payslip's disbursement status/id from the
// payment-integration layer (best-effort; gateway status is non-critical to the
// run). Payroll reads it via the Disburser interface, never the disbursements
// table — so it stays decoupled from the payment domain's schema.
func (s *PayrollService) enrichDisbursements(ctx context.Context, run *payrollifacev1.PayrollRun) {
	if s.disburser == nil || run == nil || len(run.Payslips) == 0 {
		return
	}
	ids := make([]string, 0, len(run.Payslips))
	for _, p := range run.Payslips {
		ids = append(ids, p.Id)
	}
	m, err := s.disburser.ListByReferences(ctx, "payslip", ids)
	if err != nil {
		return
	}
	for _, p := range run.Payslips {
		if rec, ok := m[p.Id]; ok {
			p.DisbursementStatus = rec.Status
			p.DisbursementId = rec.ID
		}
	}
}

// periodRevenue sums COMPLETED sales by a cashier in [from, to). Shop-wide (not
// warehouse-scoped) — payroll commission is per-employee, not per-till.
func periodRevenue(tx *gorm.DB, cashierID string, from, to time.Time) (int64, error) {
	var total int64
	err := tx.Model(&model.Sale{}).
		Where("status = ? AND cashier_user_id = ? AND completed_at >= ? AND completed_at < ?",
			saleStatusCompleted, cashierID, from, to).
		Select("COALESCE(SUM(total), 0)").Scan(&total).Error
	return total, err
}

// recomputePayslip recomputes a payslip's gross/deductions/net/employer from its
// components.
func recomputePayslip(tx *gorm.DB, payslipID string) error {
	var comps []model.PayslipComponent
	if err := tx.Where("payslip_id = ?", payslipID).Find(&comps).Error; err != nil {
		return err
	}
	var gross, deductions, employer int64
	for _, c := range comps {
		switch c.Kind {
		case kindEarning:
			gross += c.Amount
		case kindDeduction:
			deductions += c.Amount
		case kindEmployerContrib:
			employer += c.Amount
		}
	}
	return tx.Model(&model.Payslip{}).Where("id = ?", payslipID).Updates(map[string]any{
		"gross":            gross,
		"total_deductions": deductions,
		"net":              gross - deductions,
		"employer_cost":    employer,
	}).Error
}

// recomputeRun recomputes a run's rollups from its payslips.
func recomputeRun(tx *gorm.DB, runID string) error {
	var slips []model.Payslip
	if err := tx.Where("run_id = ?", runID).Find(&slips).Error; err != nil {
		return err
	}
	var g, d, n int64
	for _, p := range slips {
		g += p.Gross
		d += p.TotalDeductions
		n += p.Net
	}
	return tx.Model(&model.PayrollRun{}).Where("id = ?", runID).Updates(map[string]any{
		"total_gross":      g,
		"total_deductions": d,
		"total_net":        n,
	}).Error
}

// transition moves a run from one of allowedFrom statuses to `to`, optionally
// stamping a timestamp column (approved_at / paid_at / voided_at). Returns the
// run id so the caller can load the full response.
func (s *PayrollService) transition(ctx context.Context, id string, allowedFrom []string, to, stampCol string) error {
	if id == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run model.PayrollRun
		if err := common.RowLock(tx).Where("id = ?", id).First(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("payroll run not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if !slices.Contains(allowedFrom, run.Status) {
			return common.TokenError(connect.CodeFailedPrecondition, "payroll.invalid_transition")
		}
		updates := map[string]any{"status": to}
		if stampCol != "" {
			updates[stampCol] = time.Now()
		}
		return tx.Model(&model.PayrollRun{}).Where("id = ?", run.ID).Updates(updates).Error
	})
}

// loadComponentRun loads a component, its payslip, and the owning run. Used by
// the component-edit RPCs to enforce DRAFT-only editing.
func loadComponentRun(tx *gorm.DB, componentID string) (*model.PayslipComponent, *model.Payslip, *model.PayrollRun, error) {
	if componentID == "" {
		return nil, nil, nil, connect.NewError(connect.CodeInvalidArgument, errors.New("component_id required"))
	}
	var comp model.PayslipComponent
	if err := tx.Where("id = ?", componentID).First(&comp).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, connect.NewError(connect.CodeNotFound, errors.New("component not found"))
		}
		return nil, nil, nil, connect.NewError(connect.CodeInternal, err)
	}
	var slip model.Payslip
	if err := tx.Where("id = ?", comp.PayslipID).First(&slip).Error; err != nil {
		return nil, nil, nil, connect.NewError(connect.CodeInternal, err)
	}
	var run model.PayrollRun
	if err := tx.Where("id = ?", slip.RunID).First(&run).Error; err != nil {
		return nil, nil, nil, connect.NewError(connect.CodeInternal, err)
	}
	return &comp, &slip, &run, nil
}

// ---------- Proto mapping ----------

func unixOrZero(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.Unix()
}

func runToProto(r *model.PayrollRun, withPayslips bool) *payrollifacev1.PayrollRun {
	out := &payrollifacev1.PayrollRun{
		Id:              r.ID,
		RunNo:           common.Deref(r.RunNo),
		PeriodYear:      r.PeriodYear,
		PeriodMonth:     r.PeriodMonth,
		Status:          r.Status,
		Note:            r.Note,
		TotalGross:      r.TotalGross,
		TotalDeductions: r.TotalDeductions,
		TotalNet:        r.TotalNet,
		CreatedBy:       r.CreatedBy,
		CreatedAt:       r.CreatedAt.Unix(),
		ApprovedAt:      unixOrZero(r.ApprovedAt),
		PaidAt:          unixOrZero(r.PaidAt),
		VoidedAt:        unixOrZero(r.VoidedAt),
	}
	if withPayslips {
		for i := range r.Payslips {
			out.Payslips = append(out.Payslips, payslipToProto(&r.Payslips[i]))
		}
	}
	return out
}

func payslipToProto(p *model.Payslip) *payrollifacev1.Payslip {
	out := &payrollifacev1.Payslip{
		Id:                p.ID,
		RunId:             p.RunID,
		EmployeeId:        p.EmployeeID,
		EmployeeCode:      p.EmployeeCode,
		EmployeeName:      p.EmployeeName,
		Position:          p.Position,
		Npwp:              p.Npwp,
		PtkpStatus:        p.PtkpStatus,
		BankName:          p.BankName,
		BankAccountNumber: p.BankAccountNumber,
		BankAccountHolder: p.BankAccountHolder,
		BankChannelCode:   p.BankChannelCode,
		BaseSalary:        p.BaseSalary,
		CommissionBase:    p.CommissionBase,
		Gross:             p.Gross,
		TotalDeductions:   p.TotalDeductions,
		Net:               p.Net,
		EmployerCost:      p.EmployerCost,
	}
	for i := range p.Components {
		out.Components = append(out.Components, componentToProto(&p.Components[i]))
	}
	return out
}

func componentToProto(c *model.PayslipComponent) *payrollifacev1.PayslipComponent {
	return &payrollifacev1.PayslipComponent{
		Id:              c.ID,
		PayslipId:       c.PayslipID,
		Kind:            c.Kind,
		Code:            c.Code,
		Label:           c.Label,
		Amount:          c.Amount,
		Taxable:         c.Taxable,
		SystemGenerated: c.SystemGenerated,
		Overridden:      c.Overridden,
		SortOrder:       c.SortOrder,
	}
}

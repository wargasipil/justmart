package payroll

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// CreatePayrollRun generates a DRAFT run for a month: one payslip per active
// employee, with BASE + allowance + commission earnings and the suggested BPJS +
// PPh21 deductions. Every amount is overridable until the run is approved.
func (s *PayrollService) CreatePayrollRun(
	ctx context.Context,
	req *connect.Request[payrollifacev1.CreatePayrollRunRequest],
) (*connect.Response[payrollifacev1.CreatePayrollRunResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	year := req.Msg.PeriodYear
	month := req.Msg.PeriodMonth
	if month < 1 || month > 12 || year < 2000 || year > 3000 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.period_invalid")
	}
	periodStart := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.Local)
	periodEnd := periodStart.AddDate(0, 1, 0)

	bpjsRaw, pph21Raw, err := common.GetPayrollConfig(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	bpjsCfg := ParseBPJSConfig(bpjsRaw)
	pph21Cfg := ParsePPh21Config(pph21Raw)

	var runID string
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var live int64
		if err := tx.Model(&model.PayrollRun{}).
			Where("period_year = ? AND period_month = ? AND status <> ?", year, month, runStatusVoided).
			Count(&live).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if live > 0 {
			return common.TokenError(connect.CodeFailedPrecondition, "payroll.period_exists")
		}

		runNo, err := assignPayrollNo(tx, periodStart)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		run := model.PayrollRun{
			RunNo:       &runNo,
			PeriodYear:  year,
			PeriodMonth: month,
			Status:      runStatusDraft,
			Note:        strings.TrimSpace(req.Msg.Note),
			CreatedBy:   caller.UserID,
		}
		if err := tx.Create(&run).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		runID = run.ID

		var emps []model.Employee
		if err := tx.Preload("Allowances", func(db *gorm.DB) *gorm.DB { return db.Order("created_at") }).
			Where("active = ?", true).Order("name").Find(&emps).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		var totGross, totDed, totNet int64
		for i := range emps {
			emp := &emps[i]
			var comps []model.PayslipComponent
			sortOrder := int32(0)
			add := func(kind, code, label string, amount int64, taxable bool) {
				comps = append(comps, model.PayslipComponent{
					Kind:            kind,
					Code:            code,
					Label:           label,
					Amount:          amount,
					Taxable:         taxable,
					SystemGenerated: true,
					SortOrder:       sortOrder,
				})
				sortOrder++
			}

			add(kindEarning, codeBase, "Gaji pokok", emp.BaseSalary, true)
			for _, a := range emp.Allowances {
				if !a.Active {
					continue
				}
				add(kindEarning, codeAllowance, a.Label, a.Amount, a.Taxable)
			}

			var commissionBase int64
			if emp.UserID != nil && *emp.UserID != "" && emp.CommissionPct > 0 {
				rev, err := periodRevenue(tx, *emp.UserID, periodStart, periodEnd)
				if err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}
				commissionBase = rev
				if c := roundBps(rev, int64(emp.CommissionPct)); c > 0 {
					add(kindEarning, codeCommission, "Komisi penjualan", c, true)
				}
			}

			var gross, taxable int64
			for _, c := range comps {
				if c.Kind == kindEarning {
					gross += c.Amount
					if c.Taxable {
						taxable += c.Amount
					}
				}
			}

			statLines := ComputeStatutory(StatInput{
				Earnings:    gross,
				TaxableBase: taxable,
				BaseSalary:  emp.BaseSalary,
				PtkpStatus:  emp.PtkpStatus,
				Enrolled: BPJSEnrollment{
					Kes: emp.BpjsKesEnrolled,
					Jht: emp.BpjsJhtEnrolled,
					Jp:  emp.BpjsJpEnrolled,
					Jkk: emp.BpjsJkkEnrolled,
					Jkm: emp.BpjsJkmEnrolled,
				},
			}, bpjsCfg, pph21Cfg)
			for _, l := range statLines {
				add(l.Kind, l.Code, l.Label, l.Amount, l.Taxable)
			}

			var deductions, employer int64
			for _, c := range comps {
				switch c.Kind {
				case kindDeduction:
					deductions += c.Amount
				case kindEmployerContrib:
					employer += c.Amount
				}
			}
			net := gross - deductions

			slip := model.Payslip{
				RunID:             run.ID,
				EmployeeID:        emp.ID,
				EmployeeCode:      emp.Code,
				EmployeeName:      emp.Name,
				Position:          emp.Position,
				Npwp:              emp.Npwp,
				PtkpStatus:        emp.PtkpStatus,
				BankName:          emp.BankName,
				BankAccountNumber: emp.BankAccountNumber,
				BankAccountHolder: emp.BankAccountHolder,
				BankChannelCode:   emp.BankChannelCode,
				BaseSalary:        emp.BaseSalary,
				CommissionBase:    commissionBase,
				Gross:             gross,
				TotalDeductions:   deductions,
				Net:               net,
				EmployerCost:      employer,
			}
			if err := tx.Create(&slip).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			for j := range comps {
				comps[j].PayslipID = slip.ID
			}
			if len(comps) > 0 {
				if err := tx.Create(&comps).Error; err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}
			}
			totGross += gross
			totDed += deductions
			totNet += net
		}

		return tx.Model(&model.PayrollRun{}).Where("id = ?", run.ID).Updates(map[string]any{
			"total_gross":      totGross,
			"total_deductions": totDed,
			"total_net":        totNet,
		}).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	run, err := s.runResponse(ctx, runID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&payrollifacev1.CreatePayrollRunResponse{Run: run}), nil
}

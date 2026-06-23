package payroll_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/model"
	payrollsvc "github.com/justmart/backend/internal/service/payroll"
	"github.com/justmart/backend/internal/service/servicetest"
)

// uniq yields a process-wide unique suffix so seeded codes never collide.
var seedSeq atomic.Int64

func uniq() string { return fmt.Sprintf("%d", seedSeq.Add(1)) }

// payEnv bundles the fixtures payroll tests share: live Employee + Payroll
// services, a seeded owner (the FK target for created_by), and an
// OWNER-authenticated ctx.
type payEnv struct {
	svc     *payrollsvc.EmployeeService
	pay     *payrollsvc.PayrollService
	db      *gorm.DB
	cfg     *config.Config
	ownerID string
	ctx     context.Context
}

func newPayEnv(t *testing.T) payEnv {
	t.Helper()
	db, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, db, cfg)
	return payEnv{
		svc:     payrollsvc.NewEmployeeService(db),
		pay:     payrollsvc.NewPayrollService(db, cfg.Printer),
		db:      db,
		cfg:     cfg,
		ownerID: ownerID,
		ctx:     servicetest.OwnerCtx(context.Background(), ownerID),
	}
}

// createEmployee creates a minimal active employee with the given base salary.
func createEmployee(t *testing.T, env payEnv, name string, baseSalary int64) *payrollifacev1.Employee {
	t.Helper()
	resp, err := env.svc.CreateEmployee(env.ctx, connect.NewRequest(&payrollifacev1.CreateEmployeeRequest{
		Code:       "EMP-" + uniq(),
		Name:       name,
		BaseSalary: baseSalary,
	}))
	require.NoError(t, err)
	return resp.Msg.Employee
}

// seedSale inserts a sale row directly (bypassing the POS flow) for commission
// tests. completedAt may be nil for a DRAFT.
func seedSale(t *testing.T, db *gorm.DB, cashierID, status string, total int64, completedAt *time.Time) {
	t.Helper()
	s := model.Sale{
		CashierUserID: cashierID,
		Status:        status,
		Total:         total,
		CompletedAt:   completedAt,
	}
	require.NoError(t, db.Create(&s).Error)
}

// createRun generates a payroll run for the given period via the service.
func createRun(t *testing.T, env payEnv, year, month int32) *payrollifacev1.PayrollRun {
	t.Helper()
	resp, err := env.pay.CreatePayrollRun(env.ctx, connect.NewRequest(&payrollifacev1.CreatePayrollRunRequest{
		PeriodYear: year, PeriodMonth: month,
	}))
	require.NoError(t, err)
	return resp.Msg.Run
}

// compByCode returns the first component with the given code in a payslip.
func compByCode(p *payrollifacev1.Payslip, code string) *payrollifacev1.PayslipComponent {
	for _, c := range p.Components {
		if c.Code == code {
			return c
		}
	}
	return nil
}

package payroll_test

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

func TestCreatePayrollRun_Commission(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)

	// Employee linked to the owner user, 2.5% commission on period revenue.
	_, err := env.svc.CreateEmployee(env.ctx, connect.NewRequest(&payrollifacev1.CreateEmployeeRequest{
		Code: "EMP-C1", Name: "Cashier", BaseSalary: 3_000_000, UserId: env.ownerID, CommissionPct: 250,
	}))
	require.NoError(t, err)

	inWindow1 := time.Date(2026, 6, 5, 10, 0, 0, 0, time.Local)
	inWindow2 := time.Date(2026, 6, 20, 14, 0, 0, 0, time.Local)
	outWindow := time.Date(2026, 5, 30, 10, 0, 0, 0, time.Local)
	// In-window COMPLETED sales: 6,000,000 + 4,000,000 = 10,000,000.
	seedSale(t, env.db, env.ownerID, common.SaleStatusCompleted, 6_000_000, &inWindow1)
	seedSale(t, env.db, env.ownerID, common.SaleStatusCompleted, 4_000_000, &inWindow2)
	// Out of window + a DRAFT must NOT count.
	seedSale(t, env.db, env.ownerID, common.SaleStatusCompleted, 9_000_000, &outWindow)
	seedSale(t, env.db, env.ownerID, common.SaleStatusDraft, 5_000_000, nil)

	run := createRun(t, env, 2026, 6)
	require.Len(t, run.Payslips, 1)
	p := run.Payslips[0]
	require.Equal(t, int64(10_000_000), p.CommissionBase)

	comm := compByCode(p, "COMMISSION")
	require.NotNil(t, comm)
	require.Equal(t, int64(250_000), comm.Amount) // 2.5% of 10,000,000
	require.Equal(t, int64(3_250_000), p.Gross)    // 3M base + 250k commission
}

func TestCreatePayrollRun_NoCommissionWithoutUserLink(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	// commission_pct set but no user link -> no commission line.
	_, err := env.svc.CreateEmployee(env.ctx, connect.NewRequest(&payrollifacev1.CreateEmployeeRequest{
		Code: "EMP-C2", Name: "No link", BaseSalary: 3_000_000, CommissionPct: 500,
	}))
	require.NoError(t, err)
	run := createRun(t, env, 2026, 6)
	require.Len(t, run.Payslips, 1)
	require.Nil(t, compByCode(run.Payslips[0], "COMMISSION"))
	require.Equal(t, int64(0), run.Payslips[0].CommissionBase)
}

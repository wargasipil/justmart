package payroll_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
)

func TestCreatePayrollRun_GeneratesPayslips(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	createEmployee(t, env, "Active A", 5_000_000)
	createEmployee(t, env, "Active B", 4_000_000)
	// An inactive employee must be excluded.
	inactive := createEmployee(t, env, "Inactive", 9_000_000)
	_, err := env.svc.ArchiveEmployee(env.ctx, connect.NewRequest(&payrollifacev1.ArchiveEmployeeRequest{Id: inactive.Id}))
	require.NoError(t, err)

	run := createRun(t, env, 2026, 6)
	require.Equal(t, "PAY-2026-0001", run.RunNo)
	require.Equal(t, "DRAFT", run.Status)
	require.Len(t, run.Payslips, 2) // inactive excluded

	// Rollups equal the sum of payslips.
	var g, d, n int64
	for _, p := range run.Payslips {
		g += p.Gross
		d += p.TotalDeductions
		n += p.Net
		// Each payslip has a BASE earning component.
		require.NotNil(t, compByCode(p, "BASE"))
	}
	require.Equal(t, g, run.TotalGross)
	require.Equal(t, d, run.TotalDeductions)
	require.Equal(t, n, run.TotalNet)
	require.Equal(t, int64(9_000_000), run.TotalGross) // 5M + 4M base, no statutory (not enrolled, under PPh21 floor)
}

func TestCreatePayrollRun_Statutory(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	// Enroll BPJS Kesehatan; 1% employee deduction on 5M = 50,000.
	_, err := env.svc.CreateEmployee(env.ctx, connect.NewRequest(&payrollifacev1.CreateEmployeeRequest{
		Code: "EMP-S1", Name: "Enrolled", BaseSalary: 5_000_000, PtkpStatus: "TK0",
		BpjsKesEnrolled: true,
	}))
	require.NoError(t, err)

	run := createRun(t, env, 2026, 6)
	require.Len(t, run.Payslips, 1)
	p := run.Payslips[0]
	require.Equal(t, int64(5_000_000), p.Gross)
	kesEmp := compByCode(p, "BPJS_KES_EMP")
	require.NotNil(t, kesEmp)
	require.Equal(t, int64(50_000), kesEmp.Amount)
	require.NotNil(t, compByCode(p, "BPJS_KES_ER")) // employer line present
	require.Equal(t, int64(50_000), p.TotalDeductions)
	require.Equal(t, int64(4_950_000), p.Net)
}

func TestCreatePayrollRun_PeriodExistsAndVoidFreesSlot(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	createEmployee(t, env, "A", 1_000_000)
	run := createRun(t, env, 2026, 6)

	// A second live run for the same period is rejected.
	_, err := env.pay.CreatePayrollRun(env.ctx, connect.NewRequest(&payrollifacev1.CreatePayrollRunRequest{
		PeriodYear: 2026, PeriodMonth: 6,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	require.Equal(t, "payroll.period_exists", tokenOf(t, err))

	// Voiding it frees the slot.
	_, err = env.pay.VoidPayrollRun(env.ctx, connect.NewRequest(&payrollifacev1.VoidPayrollRunRequest{Id: run.Id}))
	require.NoError(t, err)
	run2 := createRun(t, env, 2026, 6)
	require.Equal(t, "PAY-2026-0002", run2.RunNo)
}

func TestCreatePayrollRun_PeriodInvalid(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	_, err := env.pay.CreatePayrollRun(env.ctx, connect.NewRequest(&payrollifacev1.CreatePayrollRunRequest{
		PeriodYear: 2026, PeriodMonth: 13,
	}))
	require.Error(t, err)
	require.Equal(t, "payroll.period_invalid", tokenOf(t, err))
}

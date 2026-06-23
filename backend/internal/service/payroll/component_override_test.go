package payroll_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
)

func TestUpdatePayslipComponent_OverrideRecomputes(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	createEmployee(t, env, "Emp", 5_000_000)
	run := createRun(t, env, 2026, 6)
	base := compByCode(run.Payslips[0], "BASE")
	require.NotNil(t, base)
	require.False(t, base.Overridden)

	resp, err := env.pay.UpdatePayslipComponent(env.ctx, connect.NewRequest(&payrollifacev1.UpdatePayslipComponentRequest{
		ComponentId: base.Id, Label: "Gaji pokok", Amount: 6_000_000, Taxable: true,
	}))
	require.NoError(t, err)
	p := resp.Msg.Run.Payslips[0]
	newBase := compByCode(p, "BASE")
	require.True(t, newBase.Overridden)
	require.Equal(t, int64(6_000_000), newBase.Amount)
	require.Equal(t, int64(6_000_000), p.Gross) // recomputed
	require.Equal(t, int64(6_000_000), p.Net)
	require.Equal(t, int64(6_000_000), resp.Msg.Run.TotalGross) // run rollup recomputed
}

func TestAddAndRemovePayslipComponent(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	createEmployee(t, env, "Emp", 5_000_000)
	run := createRun(t, env, 2026, 6)
	slipID := run.Payslips[0].Id

	// Add a bonus earning.
	resp, err := env.pay.AddPayslipComponent(env.ctx, connect.NewRequest(&payrollifacev1.AddPayslipComponentRequest{
		PayslipId: slipID, Kind: "EARNING", Code: "BONUS", Label: "THR", Amount: 1_000_000, Taxable: true,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(6_000_000), resp.Msg.Run.Payslips[0].Gross)
	bonus := compByCode(resp.Msg.Run.Payslips[0], "BONUS")
	require.NotNil(t, bonus)
	require.False(t, bonus.SystemGenerated)

	// Remove it.
	rem, err := env.pay.RemovePayslipComponent(env.ctx, connect.NewRequest(&payrollifacev1.RemovePayslipComponentRequest{
		ComponentId: bonus.Id,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(5_000_000), rem.Msg.Run.Payslips[0].Gross)
	require.Nil(t, compByCode(rem.Msg.Run.Payslips[0], "BONUS"))
}

func TestUpdatePayslipComponent_FrozenAfterApprove(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	createEmployee(t, env, "Emp", 5_000_000)
	run := createRun(t, env, 2026, 6)
	base := compByCode(run.Payslips[0], "BASE")

	_, err := env.pay.ApprovePayrollRun(env.ctx, connect.NewRequest(&payrollifacev1.ApprovePayrollRunRequest{Id: run.Id}))
	require.NoError(t, err)

	_, err = env.pay.UpdatePayslipComponent(env.ctx, connect.NewRequest(&payrollifacev1.UpdatePayslipComponentRequest{
		ComponentId: base.Id, Amount: 1, Taxable: true,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	require.Equal(t, "payroll.run_not_editable", tokenOf(t, err))
}

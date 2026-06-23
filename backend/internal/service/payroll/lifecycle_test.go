package payroll_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
)

func TestPayrollLifecycle_DraftApprovedPaid(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	createEmployee(t, env, "Emp", 5_000_000)
	run := createRun(t, env, 2026, 6)

	appr, err := env.pay.ApprovePayrollRun(env.ctx, connect.NewRequest(&payrollifacev1.ApprovePayrollRunRequest{Id: run.Id}))
	require.NoError(t, err)
	require.Equal(t, "APPROVED", appr.Msg.Run.Status)
	require.NotZero(t, appr.Msg.Run.ApprovedAt)

	paid, err := env.pay.MarkPayrollRunPaid(env.ctx, connect.NewRequest(&payrollifacev1.MarkPayrollRunPaidRequest{Id: run.Id}))
	require.NoError(t, err)
	require.Equal(t, "PAID", paid.Msg.Run.Status)
	require.NotZero(t, paid.Msg.Run.PaidAt)
}

func TestPayrollLifecycle_IllegalTransitions(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	createEmployee(t, env, "Emp", 5_000_000)
	run := createRun(t, env, 2026, 6)

	// MarkPaid before approval -> invalid.
	_, err := env.pay.MarkPayrollRunPaid(env.ctx, connect.NewRequest(&payrollifacev1.MarkPayrollRunPaidRequest{Id: run.Id}))
	require.Error(t, err)
	require.Equal(t, "payroll.invalid_transition", tokenOf(t, err))

	// Approve twice -> invalid the second time.
	_, err = env.pay.ApprovePayrollRun(env.ctx, connect.NewRequest(&payrollifacev1.ApprovePayrollRunRequest{Id: run.Id}))
	require.NoError(t, err)
	_, err = env.pay.ApprovePayrollRun(env.ctx, connect.NewRequest(&payrollifacev1.ApprovePayrollRunRequest{Id: run.Id}))
	require.Error(t, err)
	require.Equal(t, "payroll.invalid_transition", tokenOf(t, err))

	// Pay it, then voiding a PAID run is rejected.
	_, err = env.pay.MarkPayrollRunPaid(env.ctx, connect.NewRequest(&payrollifacev1.MarkPayrollRunPaidRequest{Id: run.Id}))
	require.NoError(t, err)
	_, err = env.pay.VoidPayrollRun(env.ctx, connect.NewRequest(&payrollifacev1.VoidPayrollRunRequest{Id: run.Id}))
	require.Error(t, err)
	require.Equal(t, "payroll.invalid_transition", tokenOf(t, err))
}

func TestPayrollLifecycle_VoidFromApproved(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	createEmployee(t, env, "Emp", 5_000_000)
	run := createRun(t, env, 2026, 6)
	_, err := env.pay.ApprovePayrollRun(env.ctx, connect.NewRequest(&payrollifacev1.ApprovePayrollRunRequest{Id: run.Id}))
	require.NoError(t, err)
	v, err := env.pay.VoidPayrollRun(env.ctx, connect.NewRequest(&payrollifacev1.VoidPayrollRunRequest{Id: run.Id}))
	require.NoError(t, err)
	require.Equal(t, "VOIDED", v.Msg.Run.Status)
}

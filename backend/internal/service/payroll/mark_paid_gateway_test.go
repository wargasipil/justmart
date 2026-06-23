package payroll_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/service/payment"
)

// fakeDisburser records CreateDisbursement calls; ListByReferences echoes them.
type fakeDisburser struct {
	created []payment.DisburseRequest
	status  string
}

func (f *fakeDisburser) CreateDisbursement(_ context.Context, req payment.DisburseRequest) (payment.DisbursementRecord, error) {
	f.created = append(f.created, req)
	return payment.DisbursementRecord{ID: "disb_" + req.ReferenceID, Status: f.statusOr(), ExternalID: "payslip_" + req.ReferenceID}, nil
}
func (f *fakeDisburser) ListByReferences(_ context.Context, _ string, refIDs []string) (map[string]payment.DisbursementRecord, error) {
	out := map[string]payment.DisbursementRecord{}
	for _, c := range f.created {
		for _, id := range refIDs {
			if c.ReferenceID == id {
				out[id] = payment.DisbursementRecord{ID: "disb_" + id, Status: f.statusOr()}
			}
		}
	}
	return out, nil
}
func (f *fakeDisburser) statusOr() string {
	if f.status == "" {
		return payment.StatusProcessing
	}
	return f.status
}

// approvedRunWithBank creates an employee (optionally with payout bank details),
// generates a run for 2026/6, and approves it. Returns the run id.
func approvedRunWithBank(t *testing.T, env payEnv, withBank bool) string {
	t.Helper()
	req := &payrollifacev1.CreateEmployeeRequest{
		Code: "EMP-" + uniq(), Name: "Budi", BaseSalary: 5_000_000,
	}
	if withBank {
		req.BankChannelCode = "ID_BCA"
		req.BankAccountNumber = "1234567890"
		req.BankAccountHolder = "Budi"
	}
	_, err := env.svc.CreateEmployee(env.ctx, connect.NewRequest(req))
	require.NoError(t, err)

	run := createRun(t, env, 2026, 6)
	_, err = env.pay.ApprovePayrollRun(env.ctx, connect.NewRequest(&payrollifacev1.ApprovePayrollRunRequest{Id: run.Id}))
	require.NoError(t, err)
	return run.Id
}

func TestMarkPaid_ManualUnchanged(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	fake := &fakeDisburser{}
	env.pay.SetDisburser(fake)
	runID := approvedRunWithBank(t, env, false)

	resp, err := env.pay.MarkPayrollRunPaid(env.ctx, connect.NewRequest(&payrollifacev1.MarkPayrollRunPaidRequest{Id: runID}))
	require.NoError(t, err)
	require.Equal(t, "PAID", resp.Msg.Run.Status)
	require.Empty(t, fake.created) // MANUAL moves no money
}

func TestMarkPaid_GatewayCreatesDisbursements(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	fake := &fakeDisburser{status: payment.StatusProcessing}
	env.pay.SetDisburser(fake)
	runID := approvedRunWithBank(t, env, true)

	resp, err := env.pay.MarkPayrollRunPaid(env.ctx, connect.NewRequest(&payrollifacev1.MarkPayrollRunPaidRequest{
		Id: runID, Method: "GATEWAY",
	}))
	require.NoError(t, err)
	require.Equal(t, "PAID", resp.Msg.Run.Status)
	require.Len(t, fake.created, 1)
	require.Equal(t, "ID_BCA", fake.created[0].ChannelCode)
	require.Equal(t, int64(5_000_000), fake.created[0].Amount)
	require.Equal(t, "payslip", fake.created[0].ReferenceType)
	// Per-payslip disbursement status surfaced on the run.
	require.Equal(t, payment.StatusProcessing, resp.Msg.Run.Payslips[0].DisbursementStatus)
}

func TestMarkPaid_GatewayMissingChannelCode(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	fake := &fakeDisburser{}
	env.pay.SetDisburser(fake)
	runID := approvedRunWithBank(t, env, false) // no bank channel code

	_, err := env.pay.MarkPayrollRunPaid(env.ctx, connect.NewRequest(&payrollifacev1.MarkPayrollRunPaidRequest{
		Id: runID, Method: "GATEWAY",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	require.Equal(t, "payroll.missing_channel_code", tokenOf(t, err))
	require.Empty(t, fake.created)

	// Run stays APPROVED (no money moved).
	got, err := env.pay.GetPayrollRun(env.ctx, connect.NewRequest(&payrollifacev1.GetPayrollRunRequest{Id: runID}))
	require.NoError(t, err)
	require.Equal(t, "APPROVED", got.Msg.Run.Status)
}

func TestMarkPaid_GatewayNoDisburser(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t) // no SetDisburser
	runID := approvedRunWithBank(t, env, true)
	_, err := env.pay.MarkPayrollRunPaid(env.ctx, connect.NewRequest(&payrollifacev1.MarkPayrollRunPaidRequest{
		Id: runID, Method: "GATEWAY",
	}))
	require.Error(t, err)
	require.Equal(t, "payment.no_provider", tokenOf(t, err))
}

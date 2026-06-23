package payment_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/justmart/backend/internal/service/payment"
)

func TestCreateDisbursement_ProcessingAndIdempotent(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	env.fake.result = payment.DisburseResult{Status: payment.StatusProcessing}

	rec, err := env.svc.CreateDisbursement(env.ctx, env.disbReq("ps1", 5_000_000))
	require.NoError(t, err)
	require.Equal(t, payment.StatusProcessing, rec.Status)
	require.Equal(t, "payslip_ps1", rec.ExternalID)
	require.Equal(t, "fake", rec.Provider)
	require.Equal(t, 1, env.fake.disburseCalls)

	// Same reference → returns the existing record, no second submit.
	rec2, err := env.svc.CreateDisbursement(env.ctx, env.disbReq("ps1", 5_000_000))
	require.NoError(t, err)
	require.Equal(t, rec.ID, rec2.ID)
	require.Equal(t, 1, env.fake.disburseCalls)
}

func TestCreateDisbursement_Completed(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	env.fake.result = payment.DisburseResult{Status: payment.StatusCompleted}
	rec, err := env.svc.CreateDisbursement(env.ctx, env.disbReq("ps1", 1000))
	require.NoError(t, err)
	require.Equal(t, payment.StatusCompleted, rec.Status)
	d := env.loadDisb(t, rec.ID)
	require.NotNil(t, d.CompletedAt) // stamped on COMPLETED
}

func TestCreateDisbursement_ProviderDeclineIsFailed(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	env.fake.disburseErr = errors.New("insufficient balance")
	rec, err := env.svc.CreateDisbursement(env.ctx, env.disbReq("ps1", 1000))
	require.NoError(t, err) // a decline is an outcome, not a Go error
	require.Equal(t, payment.StatusFailed, rec.Status)
	d := env.loadDisb(t, rec.ID)
	require.Contains(t, d.FailureReason, "insufficient balance")
}

func TestCreateDisbursement_NoProvider(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	// Disable the active provider.
	resetActive(t, env)
	_, err := env.svc.CreateDisbursement(env.ctx, env.disbReq("ps1", 1000))
	require.Error(t, err)
	require.Equal(t, "payment.no_provider", tokenOf(t, err))
}

func TestListByReferences(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	_, err := env.svc.CreateDisbursement(env.ctx, env.disbReq("ps1", 1000))
	require.NoError(t, err)
	_, err = env.svc.CreateDisbursement(env.ctx, env.disbReq("ps2", 2000))
	require.NoError(t, err)

	m, err := env.svc.ListByReferences(env.ctx, "payslip", []string{"ps1", "ps2", "ps3"})
	require.NoError(t, err)
	require.Len(t, m, 2)
	require.Contains(t, m, "ps1")
	require.Contains(t, m, "ps2")
	require.NotContains(t, m, "ps3")
}

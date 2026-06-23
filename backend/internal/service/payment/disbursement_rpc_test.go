package payment_test

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	paymentifacev1 "github.com/justmart/backend/gen/payment_iface/v1"
	"github.com/justmart/backend/internal/service/payment"
)

func TestListAndGetDisbursements(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	rec, err := env.svc.CreateDisbursement(env.ctx, env.disbReq("ps1", 1000))
	require.NoError(t, err)

	list, err := env.svc.ListDisbursements(env.ctx, connect.NewRequest(&paymentifacev1.ListDisbursementsRequest{
		ReferenceType: "payslip",
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), list.Msg.Total)
	require.Len(t, list.Msg.Disbursements, 1)

	got, err := env.svc.GetDisbursement(env.ctx, connect.NewRequest(&paymentifacev1.GetDisbursementRequest{Id: rec.ID}))
	require.NoError(t, err)
	require.Equal(t, rec.ID, got.Msg.Disbursement.Id)
	require.Equal(t, "ID_BCA", got.Msg.Disbursement.ChannelCode)
}

func TestRetryDisbursement(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	env.fake.disburseErr = errors.New("declined")
	rec, err := env.svc.CreateDisbursement(env.ctx, env.disbReq("ps1", 1000))
	require.NoError(t, err)
	require.Equal(t, payment.StatusFailed, rec.Status)

	// Retry now succeeds.
	env.fake.disburseErr = nil
	env.fake.result = payment.DisburseResult{Status: payment.StatusCompleted}
	resp, err := env.svc.RetryDisbursement(env.ctx, connect.NewRequest(&paymentifacev1.RetryDisbursementRequest{Id: rec.ID}))
	require.NoError(t, err)
	require.Equal(t, payment.StatusCompleted, resp.Msg.Disbursement.Status)

	// Retrying a non-FAILED row is rejected.
	_, err = env.svc.RetryDisbursement(env.ctx, connect.NewRequest(&paymentifacev1.RetryDisbursementRequest{Id: rec.ID}))
	require.Error(t, err)
	require.Equal(t, "payment.retry_not_failed", tokenOf(t, err))
}

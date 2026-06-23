package payment_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/justmart/backend/internal/service/payment"
)

func hdr(token string) http.Header {
	h := http.Header{}
	h.Set("x-callback-token", token)
	return h
}

func TestHandleWebhook_CompletesAndIdempotent(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	env.fake.webhookToken = "whtok"
	env.fake.result = payment.DisburseResult{Status: payment.StatusProcessing}
	rec, err := env.svc.CreateDisbursement(env.ctx, env.disbReq("ps1", 1000))
	require.NoError(t, err)
	require.Equal(t, payment.StatusProcessing, rec.Status)

	body := []byte(`{"external_id":"payslip_ps1","status":"COMPLETED"}`)
	require.NoError(t, env.svc.HandleWebhook(env.ctx, "fake", hdr("whtok"), body))
	d := env.loadDisb(t, rec.ID)
	require.Equal(t, payment.StatusCompleted, d.Status)
	require.NotNil(t, d.CompletedAt)

	// Terminal → a later FAILED event is ignored.
	require.NoError(t, env.svc.HandleWebhook(env.ctx, "fake", hdr("whtok"),
		[]byte(`{"external_id":"payslip_ps1","status":"FAILED"}`)))
	require.Equal(t, payment.StatusCompleted, env.loadDisb(t, rec.ID).Status)
}

func TestHandleWebhook_RejectsBadToken(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	env.fake.webhookToken = "whtok"
	rec, err := env.svc.CreateDisbursement(env.ctx, env.disbReq("ps1", 1000))
	require.NoError(t, err)

	err = env.svc.HandleWebhook(env.ctx, "fake", hdr("WRONG"),
		[]byte(`{"external_id":"payslip_ps1","status":"COMPLETED"}`))
	require.ErrorIs(t, err, payment.ErrWebhookUnauthorized)
	require.Equal(t, payment.StatusProcessing, env.loadDisb(t, rec.ID).Status) // unchanged
}

func TestHandleWebhook_UnknownEvent(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	env.fake.webhookToken = "whtok"
	err := env.svc.HandleWebhook(env.ctx, "fake", hdr("whtok"),
		[]byte(`{"external_id":"payslip_nope","status":"COMPLETED"}`))
	require.ErrorIs(t, err, payment.ErrWebhookUnknownEvent)
}

func TestHandleWebhook_UnknownProvider(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	err := env.svc.HandleWebhook(env.ctx, "nope", hdr("x"), []byte(`{}`))
	require.True(t, errors.Is(err, payment.ErrWebhookUnauthorized))
}

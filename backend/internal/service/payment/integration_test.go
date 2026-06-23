package payment_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	paymentifacev1 "github.com/justmart/backend/gen/payment_iface/v1"
)

func findProvider(resp *paymentifacev1.GetIntegrationStatusResponse, name string) *paymentifacev1.ProviderStatus {
	for _, p := range resp.Providers {
		if p.Provider == name {
			return p
		}
	}
	return nil
}

func TestApplyProviderCredentials_Xendit(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	resp, err := env.svc.ApplyProviderCredentials(env.ctx, connect.NewRequest(&paymentifacev1.ApplyProviderCredentialsRequest{
		Provider:    "xendit",
		Credentials: map[string]string{"api_key": "xnd_development_ABCD1234", "webhook_token": "whtok"},
	}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Status.Configured)
	require.True(t, resp.Msg.Status.WebhookSet)
	require.Contains(t, resp.Msg.Status.KeyMasked, "1234")
	require.NotContains(t, resp.Msg.Status.KeyMasked, "ABCD1234") // full key never leaked

	st, err := env.svc.GetIntegrationStatus(env.ctx, connect.NewRequest(&paymentifacev1.GetIntegrationStatusRequest{}))
	require.NoError(t, err)
	x := findProvider(st.Msg, "xendit")
	require.NotNil(t, x)
	require.True(t, x.Configured)
	require.Equal(t, "/webhooks/xendit", x.WebhookUrlPath)
}

func TestApplyProviderCredentials_IncompleteRejected(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	_, err := env.svc.ApplyProviderCredentials(env.ctx, connect.NewRequest(&paymentifacev1.ApplyProviderCredentialsRequest{
		Provider:    "xendit",
		Credentials: map[string]string{"api_key": "xnd_x"}, // missing webhook_token
	}))
	require.Error(t, err)
	require.Equal(t, "payment.credentials_incomplete", tokenOf(t, err))
}

func TestApplyProviderCredentials_UnknownProvider(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	_, err := env.svc.ApplyProviderCredentials(env.ctx, connect.NewRequest(&paymentifacev1.ApplyProviderCredentialsRequest{
		Provider:    "stripe",
		Credentials: map[string]string{"api_key": "x"},
	}))
	require.Error(t, err)
	require.Equal(t, "payment.unknown_provider", tokenOf(t, err))
}

func TestSetActiveProvider(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	resp, err := env.svc.SetActiveProvider(env.ctx, connect.NewRequest(&paymentifacev1.SetActiveProviderRequest{Provider: "xendit"}))
	require.NoError(t, err)
	require.Equal(t, "xendit", resp.Msg.ActiveProvider)

	_, err = env.svc.SetActiveProvider(env.ctx, connect.NewRequest(&paymentifacev1.SetActiveProviderRequest{Provider: "nope"}))
	require.Error(t, err)
	require.Equal(t, "payment.unknown_provider", tokenOf(t, err))
}

package payment

import (
	"context"
	"sort"

	"connectrpc.com/connect"

	paymentifacev1 "github.com/justmart/backend/gen/payment_iface/v1"
)

func (s *Service) GetIntegrationStatus(
	ctx context.Context,
	_ *connect.Request[paymentifacev1.GetIntegrationStatusRequest],
) (*connect.Response[paymentifacev1.GetIntegrationStatusResponse], error) {
	active, err := s.activeProviderName(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	names := make([]string, 0, len(s.providers))
	for name := range s.providers {
		names = append(names, name)
	}
	sort.Strings(names)

	providers := make([]*paymentifacev1.ProviderStatus, 0, len(names))
	for _, name := range names {
		st, err := s.statusOf(ctx, s.providers[name])
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		providers = append(providers, st)
	}
	return connect.NewResponse(&paymentifacev1.GetIntegrationStatusResponse{
		ActiveProvider: active,
		Providers:      providers,
	}), nil
}

// statusOf builds a provider-agnostic status view (never returns the raw key).
func (s *Service) statusOf(ctx context.Context, prov Provider) (*paymentifacev1.ProviderStatus, error) {
	creds, err := s.providerCreds(ctx, prov.Name())
	if err != nil {
		return nil, err
	}
	required := prov.RequiredCredentialKeys()
	configured := len(required) > 0
	for _, k := range required {
		if creds[k] == "" {
			configured = false
		}
	}
	st := &paymentifacev1.ProviderStatus{
		Provider:       prov.Name(),
		Configured:     configured,
		WebhookSet:     creds["webhook_token"] != "",
		WebhookUrlPath: "/webhooks/" + prov.Name(),
	}
	if len(required) > 0 {
		st.KeyMasked = maskKey(creds[required[0]])
	}
	return st, nil
}

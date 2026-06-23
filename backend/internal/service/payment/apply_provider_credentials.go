package payment

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	paymentifacev1 "github.com/justmart/backend/gen/payment_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

func (s *Service) ApplyProviderCredentials(
	ctx context.Context,
	req *connect.Request[paymentifacev1.ApplyProviderCredentialsRequest],
) (*connect.Response[paymentifacev1.ApplyProviderCredentialsResponse], error) {
	provider := strings.TrimSpace(req.Msg.Provider)
	prov, ok := s.providerByName(provider)
	if !ok {
		return nil, common.TokenError(connect.CodeInvalidArgument, "payment.unknown_provider")
	}
	creds := req.Msg.Credentials
	for _, k := range prov.RequiredCredentialKeys() {
		if strings.TrimSpace(creds[k]) == "" {
			return nil, common.TokenError(connect.CodeInvalidArgument, "payment.credentials_incomplete")
		}
	}

	switch provider {
	case ProviderXendit:
		if err := common.SetXenditCredentials(ctx, s.db,
			strings.TrimSpace(creds["api_key"]), strings.TrimSpace(creds["webhook_token"])); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	default:
		return nil, common.TokenError(connect.CodeInvalidArgument, "payment.unknown_provider")
	}

	st, err := s.statusOf(ctx, prov)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&paymentifacev1.ApplyProviderCredentialsResponse{Status: st}), nil
}

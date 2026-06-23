package payment

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	paymentifacev1 "github.com/justmart/backend/gen/payment_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

func (s *Service) SetActiveProvider(
	ctx context.Context,
	req *connect.Request[paymentifacev1.SetActiveProviderRequest],
) (*connect.Response[paymentifacev1.SetActiveProviderResponse], error) {
	provider := strings.TrimSpace(req.Msg.Provider)
	if provider != "" {
		if _, ok := s.providerByName(provider); !ok {
			return nil, common.TokenError(connect.CodeInvalidArgument, "payment.unknown_provider")
		}
	}
	if err := common.SetActiveProvider(ctx, s.db, provider); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&paymentifacev1.SetActiveProviderResponse{ActiveProvider: provider}), nil
}

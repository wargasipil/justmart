package payment

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/service/common"
)

// Service owns the disbursement domain + the provider registry. It satisfies the
// Disburser interface (used in-process by payroll) AND the two generated Connect
// service interfaces (PaymentIntegrationService + DisbursementService).
type Service struct {
	db        *gorm.DB
	cfg       config.Payment
	providers map[string]Provider
}

func NewService(db *gorm.DB, cfg config.Payment) *Service {
	s := &Service{db: db, cfg: cfg, providers: map[string]Provider{}}
	s.RegisterProvider(newXenditProvider())
	return s
}

// RegisterProvider adds a gateway adapter to the registry (tests inject fakes).
func (s *Service) RegisterProvider(p Provider) { s.providers[p.Name()] = p }

// activeProviderName: config/env wins over the UI-applied setting.
func (s *Service) activeProviderName(ctx context.Context) (string, error) {
	if s.cfg.ActiveProvider != "" {
		return s.cfg.ActiveProvider, nil
	}
	return common.GetActiveProvider(ctx, s.db)
}

// activeProvider resolves the configured gateway adapter, or a FailedPrecondition
// token when none is configured (caller maps it).
func (s *Service) activeProvider(ctx context.Context) (Provider, error) {
	name, err := s.activeProviderName(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if name == "" {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "payment.no_provider")
	}
	p, ok := s.providers[name]
	if !ok {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "payment.no_provider")
	}
	return p, nil
}

// providerByName resolves a registered adapter by name (webhook routing).
func (s *Service) providerByName(name string) (Provider, bool) {
	p, ok := s.providers[name]
	return p, ok
}

// providerCreds returns a provider's effective credentials — config/env over the
// values applied via Settings ▸ Integrations (mirrors the license precedence).
func (s *Service) providerCreds(ctx context.Context, provider string) (map[string]string, error) {
	switch provider {
	case ProviderXendit:
		apiKey, webhookToken, err := common.GetXenditCredentials(ctx, s.db)
		if err != nil {
			return nil, err
		}
		if s.cfg.XenditAPIKey != "" {
			apiKey = s.cfg.XenditAPIKey
		}
		if s.cfg.XenditWebhookToken != "" {
			webhookToken = s.cfg.XenditWebhookToken
		}
		return map[string]string{"api_key": apiKey, "webhook_token": webhookToken}, nil
	default:
		return map[string]string{}, nil
	}
}

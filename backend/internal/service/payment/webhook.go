package payment

import (
	"context"
	"errors"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/justmart/backend/internal/model"
)

// Webhook outcome sentinels so the root-mux HTTP handler can map to status codes.
var (
	ErrWebhookUnauthorized = errors.New("webhook token mismatch")
	ErrWebhookUnknownEvent = errors.New("webhook references an unknown disbursement")
)

// HandleWebhook verifies + applies a gateway callback. It is invoked by the
// plain HTTP handler mounted on the root mux (NOT a Connect RPC). Idempotent:
// duplicate deliveries and post-terminal events are no-ops.
func (s *Service) HandleWebhook(ctx context.Context, providerName string, headers http.Header, body []byte) error {
	prov, ok := s.providerByName(providerName)
	if !ok {
		return ErrWebhookUnauthorized
	}
	creds, err := s.providerCreds(ctx, providerName)
	if err != nil {
		return err
	}
	if !prov.VerifyWebhook(creds, headers.Get("x-callback-token")) {
		return ErrWebhookUnauthorized
	}
	ev, err := prov.ParseWebhook(body)
	if err != nil {
		return err
	}

	// Resolve the disbursement by external_id, falling back to (provider, provider_ref).
	var d model.Disbursement
	q := s.db.WithContext(ctx)
	if ev.ExternalID != "" {
		err = q.Where("external_id = ?", ev.ExternalID).First(&d).Error
	} else if ev.ProviderRef != "" {
		err = q.Where("provider = ? AND provider_ref = ?", providerName, ev.ProviderRef).First(&d).Error
	} else {
		return ErrWebhookUnknownEvent
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrWebhookUnknownEvent
	}
	if err != nil {
		return err
	}

	// Idempotency: ignore once terminal, or when nothing changes.
	if d.Status == StatusCompleted || d.Status == StatusFailed || d.Status == StatusVoided {
		return nil
	}
	updates := map[string]any{"status": ev.Status}
	if ev.ProviderRef != "" {
		updates["provider_ref"] = ev.ProviderRef
	}
	if ev.FailureReason != "" {
		updates["failure_reason"] = ev.FailureReason
	}
	if ev.Status == StatusCompleted {
		updates["completed_at"] = time.Now()
	}
	return s.db.WithContext(ctx).Model(&model.Disbursement{}).Where("id = ?", d.ID).Updates(updates).Error
}

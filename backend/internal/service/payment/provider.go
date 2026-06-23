// Package payment is the provider-agnostic payment-integration abstraction layer.
// It owns the disbursement (money-out) domain — records, state machine, RPCs —
// and dispatches to a swappable gateway Provider (Xendit now, Midtrans later).
// Callers (payroll) depend ONLY on the Disburser interface; this package never
// imports a caller domain (it links back by opaque reference strings).
package payment

import "context"

// Disbursement status values (stored as plain strings, like payroll statuses, so
// adding a state never needs a proto/schema enum change).
const (
	StatusPending    = "PENDING"
	StatusProcessing = "PROCESSING"
	StatusCompleted  = "COMPLETED"
	StatusFailed     = "FAILED"
	StatusVoided     = "VOIDED"
)

// ProviderXendit is the registry key + stored `provider` value for Xendit.
const ProviderXendit = "xendit"

// DisburseInput is everything an adapter needs to submit one payout.
type DisburseInput struct {
	ExternalID    string // our idempotency key (also Xendit reference_id + Idempotency-Key)
	ChannelCode   string // "ID_BCA", "ID_OVO", ...
	AccountNumber string
	AccountHolder string
	Amount        int64  // minor units (whole IDR)
	Currency      string // "IDR"
}

// DisburseResult is the adapter's submission outcome, mapped to our status enum.
type DisburseResult struct {
	ProviderRef   string // gateway-assigned id
	Status        string // PROCESSING | COMPLETED | FAILED
	FailureReason string
}

// WebhookEvent is a parsed, normalized gateway callback.
type WebhookEvent struct {
	ExternalID    string `json:"external_id"`
	ProviderRef   string `json:"provider_ref"`
	Status        string `json:"status"`
	FailureReason string `json:"failure_reason"`
}

// Provider is the gateway adapter contract. Adapters are stateless: the Service
// resolves effective credentials (config/env over settings) and passes them in,
// so a UI credential edit takes effect on the next call with no restart.
type Provider interface {
	Name() string
	RequiredCredentialKeys() []string
	Disburse(ctx context.Context, creds map[string]string, in DisburseInput) (DisburseResult, error)
	VerifyWebhook(creds map[string]string, token string) bool
	ParseWebhook(body []byte) (WebhookEvent, error)
}

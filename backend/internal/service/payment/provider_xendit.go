package payment

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"strings"

	xendit "github.com/xendit/xendit-go/v7"
	xpayout "github.com/xendit/xendit-go/v7/payout"
)

// xenditProvider is the first concrete gateway adapter — Xendit Payouts via the
// official xendit-go/v7 SDK, isolated behind the Provider interface so payroll
// and a future Midtrans adapter are unaffected.
type xenditProvider struct{}

func newXenditProvider() *xenditProvider { return &xenditProvider{} }

func (x *xenditProvider) Name() string { return ProviderXendit }

func (x *xenditProvider) RequiredCredentialKeys() []string {
	return []string{"api_key", "webhook_token"}
}

func (x *xenditProvider) Disburse(ctx context.Context, creds map[string]string, in DisburseInput) (DisburseResult, error) {
	apiKey := creds["api_key"]
	if apiKey == "" {
		return DisburseResult{}, errors.New("xendit api key not configured")
	}
	client := xendit.NewClient(apiKey)

	props := *xpayout.NewDigitalPayoutChannelProperties(in.AccountNumber)
	if in.AccountHolder != "" {
		props.SetAccountHolderName(in.AccountHolder)
	}
	// Our minor units are whole rupiah, so the float amount is exact for IDR.
	req := *xpayout.NewCreatePayoutRequest(in.ExternalID, in.ChannelCode, props, float32(in.Amount), in.Currency)

	resp, _, sdkErr := client.PayoutApi.
		CreatePayout(ctx).
		CreatePayoutRequest(req).
		IdempotencyKey(in.ExternalID). // gateway-side dedupe on retries
		Execute()
	if sdkErr != nil {
		// A gateway decline is an OUTCOME (FAILED), not an internal Go error.
		return DisburseResult{Status: StatusFailed, FailureReason: sdkErr.Error()}, nil
	}
	if resp == nil || resp.Payout == nil {
		return DisburseResult{Status: StatusProcessing}, nil
	}
	p := resp.Payout
	res := DisburseResult{ProviderRef: p.Id, Status: mapPayoutStatus(p.Status)}
	if p.FailureCode != nil {
		res.FailureReason = *p.FailureCode
	}
	return res, nil
}

func (x *xenditProvider) VerifyWebhook(creds map[string]string, token string) bool {
	want := creds["webhook_token"]
	if want == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(want), []byte(token)) == 1
}

// xenditWebhookPayout is the payout fields Xendit sends; the callback may deliver
// them flat or wrapped under "data".
type xenditWebhookPayout struct {
	Id          string  `json:"id"`
	ReferenceId string  `json:"reference_id"`
	Status      string  `json:"status"`
	FailureCode *string `json:"failure_code"`
}

type xenditWebhookEnvelope struct {
	Data                *xenditWebhookPayout `json:"data"`
	xenditWebhookPayout                      // embedded flat fields
}

func (x *xenditProvider) ParseWebhook(body []byte) (WebhookEvent, error) {
	var env xenditWebhookEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return WebhookEvent{}, err
	}
	p := env.xenditWebhookPayout
	if env.Data != nil && (env.Data.ReferenceId != "" || env.Data.Id != "") {
		p = *env.Data
	}
	ev := WebhookEvent{
		ExternalID:  p.ReferenceId,
		ProviderRef: p.Id,
		Status:      mapPayoutStatus(p.Status),
	}
	if p.FailureCode != nil {
		ev.FailureReason = *p.FailureCode
	}
	return ev, nil
}

// mapPayoutStatus maps Xendit payout statuses onto our abstract enum.
func mapPayoutStatus(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "SUCCEEDED", "COMPLETED":
		return StatusCompleted
	case "FAILED", "CANCELLED", "REVERSED", "REFUNDED":
		return StatusFailed
	default: // ACCEPTED, REQUESTED, PENDING, LOCKED, ...
		return StatusProcessing
	}
}

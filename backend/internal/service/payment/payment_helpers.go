package payment

import (
	"fmt"

	paymentifacev1 "github.com/justmart/backend/gen/payment_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// externalID is our idempotency key for a reference (e.g. payslip). One
// disbursement per reference; a retry appends a "_r<n>" suffix.
func externalID(refType, refID string) string {
	return fmt.Sprintf("%s_%s", refType, refID)
}

// maskKey returns a non-secret display hint for an API key, e.g. "xnd_…AB12".
func maskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return "…" + key
	}
	prefix := ""
	if i := indexByte(key, '_'); i > 0 && i < 8 {
		prefix = key[:i+1]
	}
	return prefix + "…" + key[len(key)-4:]
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func disbursementToProto(d *model.Disbursement) *paymentifacev1.Disbursement {
	out := &paymentifacev1.Disbursement{
		Id:            d.ID,
		Provider:      d.Provider,
		ExternalId:    d.ExternalID,
		ReferenceType: d.ReferenceType,
		ReferenceId:   d.ReferenceID,
		ChannelCode:   d.ChannelCode,
		AccountNumber: d.AccountNumber,
		AccountHolder: d.AccountHolder,
		Amount:        d.Amount,
		Currency:      d.Currency,
		Status:        d.Status,
		ProviderRef:   d.ProviderRef,
		FailureReason: d.FailureReason,
		CreatedBy:     d.CreatedBy,
		CreatedAt:     d.CreatedAt.Unix(),
	}
	if d.CompletedAt != nil {
		out.CompletedAt = d.CompletedAt.Unix()
	}
	return out
}

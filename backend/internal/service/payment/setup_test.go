package payment_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/service/payment"
	"github.com/justmart/backend/internal/service/servicetest"
)

// fakeProvider is a deterministic, network-free Provider for tests.
type fakeProvider struct {
	result        payment.DisburseResult
	disburseErr   error
	webhookToken  string
	disburseCalls int
}

func (f *fakeProvider) Name() string                      { return "fake" }
func (f *fakeProvider) RequiredCredentialKeys() []string  { return nil }
func (f *fakeProvider) Disburse(_ context.Context, _ map[string]string, in payment.DisburseInput) (payment.DisburseResult, error) {
	f.disburseCalls++
	if f.disburseErr != nil {
		return payment.DisburseResult{}, f.disburseErr
	}
	res := f.result
	if res.Status == "" {
		res.Status = payment.StatusProcessing
	}
	if res.ProviderRef == "" {
		res.ProviderRef = "fakeref_" + in.ExternalID
	}
	return res, nil
}
func (f *fakeProvider) VerifyWebhook(_ map[string]string, token string) bool {
	return f.webhookToken != "" && token == f.webhookToken
}
func (f *fakeProvider) ParseWebhook(body []byte) (payment.WebhookEvent, error) {
	var ev payment.WebhookEvent
	err := json.Unmarshal(body, &ev)
	return ev, err
}

type payEnv struct {
	svc     *payment.Service
	db      *gorm.DB
	fake    *fakeProvider
	ownerID string
	ctx     context.Context
}

func newPayEnv(t *testing.T) payEnv {
	t.Helper()
	db, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, db, cfg)
	svc := payment.NewService(db, config.Payment{})
	fake := &fakeProvider{}
	svc.RegisterProvider(fake)
	require.NoError(t, common.SetActiveProvider(context.Background(), db, "fake"))
	return payEnv{
		svc:     svc,
		db:      db,
		fake:    fake,
		ownerID: ownerID,
		ctx:     servicetest.OwnerCtx(context.Background(), ownerID),
	}
}

// disbReq builds a minimal payslip disbursement request.
func (e payEnv) disbReq(refID string, amount int64) payment.DisburseRequest {
	return payment.DisburseRequest{
		ReferenceType: "payslip",
		ReferenceID:   refID,
		ChannelCode:   "ID_BCA",
		AccountNumber: "1234567890",
		AccountHolder: "Budi",
		Amount:        amount,
		CreatedBy:     e.ownerID,
	}
}

func (e payEnv) loadDisb(t *testing.T, id string) model.Disbursement {
	t.Helper()
	var d model.Disbursement
	require.NoError(t, e.db.Where("id = ?", id).First(&d).Error)
	return d
}

// resetActive clears the active provider (manual-only).
func resetActive(t *testing.T, e payEnv) {
	t.Helper()
	require.NoError(t, common.SetActiveProvider(context.Background(), e.db, ""))
}

func tokenOf(t *testing.T, err error) string {
	t.Helper()
	var ce *connect.Error
	require.True(t, errors.As(err, &ce), "expected connect error, got %v", err)
	return ce.Message()
}

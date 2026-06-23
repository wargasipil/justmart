package payment

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/model"
)

// DisburseRequest is what a caller (payroll) hands to CreateDisbursement.
type DisburseRequest struct {
	ReferenceType string // "payslip"
	ReferenceID   string // payslip id
	ChannelCode   string
	AccountNumber string
	AccountHolder string
	Amount        int64  // minor units
	Currency      string // "" defaults to "IDR"
	CreatedBy     string // user id (audit)
}

// DisbursementRecord is the minimal view returned to callers (payroll). Callers
// never see the full model.Disbursement / the disbursements table.
type DisbursementRecord struct {
	ID            string
	Provider      string
	ExternalID    string
	Status        string
	ProviderRef   string
	FailureReason string
}

// Disburser is the ONLY surface payroll depends on. *Service implements it.
type Disburser interface {
	// CreateDisbursement inserts a PENDING record (idempotent by reference) and
	// submits it to the active provider. It does NOT hold a DB transaction across
	// the provider HTTP call. A provider decline yields a FAILED record (not a Go
	// error); a Go error means an internal/precondition failure (no provider, DB).
	CreateDisbursement(ctx context.Context, req DisburseRequest) (DisbursementRecord, error)
	// ListByReferences returns the latest disbursement per reference id (so a
	// caller can surface per-row status without touching the disbursements table).
	ListByReferences(ctx context.Context, refType string, refIDs []string) (map[string]DisbursementRecord, error)
}

func recordOf(d *model.Disbursement) DisbursementRecord {
	return DisbursementRecord{
		ID:            d.ID,
		Provider:      d.Provider,
		ExternalID:    d.ExternalID,
		Status:        d.Status,
		ProviderRef:   d.ProviderRef,
		FailureReason: d.FailureReason,
	}
}

func (s *Service) CreateDisbursement(ctx context.Context, req DisburseRequest) (DisbursementRecord, error) {
	prov, err := s.activeProvider(ctx)
	if err != nil {
		return DisbursementRecord{}, err
	}
	currency := req.Currency
	if currency == "" {
		currency = "IDR"
	}
	extID := externalID(req.ReferenceType, req.ReferenceID)

	// Idempotency: never create a second disbursement for the same reference.
	var existing model.Disbursement
	err = s.db.WithContext(ctx).Where("external_id = ?", extID).First(&existing).Error
	if err == nil {
		return recordOf(&existing), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return DisbursementRecord{}, connect.NewError(connect.CodeInternal, err)
	}

	d := model.Disbursement{
		Provider:      prov.Name(),
		ExternalID:    extID,
		ReferenceType: req.ReferenceType,
		ReferenceID:   req.ReferenceID,
		ChannelCode:   req.ChannelCode,
		AccountNumber: req.AccountNumber,
		AccountHolder: req.AccountHolder,
		Amount:        req.Amount,
		Currency:      currency,
		Status:        StatusPending,
		CreatedBy:     req.CreatedBy,
	}
	if err := s.db.WithContext(ctx).Create(&d).Error; err != nil {
		// Lost a race on the unique external_id → return the winner.
		if e2 := s.db.WithContext(ctx).Where("external_id = ?", extID).First(&existing).Error; e2 == nil {
			return recordOf(&existing), nil
		}
		return DisbursementRecord{}, connect.NewError(connect.CodeInternal, err)
	}

	// Submit to the gateway OUTSIDE any DB transaction.
	s.submit(ctx, prov, &d)
	return recordOf(&d), nil
}

// submit sends a PENDING/FAILED row to the provider and persists the outcome.
func (s *Service) submit(ctx context.Context, prov Provider, d *model.Disbursement) {
	creds, err := s.providerCreds(ctx, prov.Name())
	if err != nil {
		s.markFailed(ctx, d, "credentials: "+err.Error())
		return
	}
	res, err := prov.Disburse(ctx, creds, DisburseInput{
		ExternalID:    d.ExternalID,
		ChannelCode:   d.ChannelCode,
		AccountNumber: d.AccountNumber,
		AccountHolder: d.AccountHolder,
		Amount:        d.Amount,
		Currency:      d.Currency,
	})
	if err != nil {
		s.markFailed(ctx, d, err.Error())
		return
	}
	d.Status = res.Status
	d.ProviderRef = res.ProviderRef
	d.FailureReason = res.FailureReason
	updates := map[string]any{
		"status":         d.Status,
		"provider_ref":   d.ProviderRef,
		"failure_reason": d.FailureReason,
	}
	if d.Status == StatusCompleted {
		now := time.Now()
		d.CompletedAt = &now
		updates["completed_at"] = now
	}
	_ = s.db.WithContext(ctx).Model(&model.Disbursement{}).Where("id = ?", d.ID).Updates(updates).Error
}

func (s *Service) markFailed(ctx context.Context, d *model.Disbursement, reason string) {
	d.Status = StatusFailed
	d.FailureReason = reason
	_ = s.db.WithContext(ctx).Model(&model.Disbursement{}).Where("id = ?", d.ID).
		Updates(map[string]any{"status": StatusFailed, "failure_reason": reason}).Error
}

func (s *Service) ListByReferences(ctx context.Context, refType string, refIDs []string) (map[string]DisbursementRecord, error) {
	out := make(map[string]DisbursementRecord, len(refIDs))
	if len(refIDs) == 0 {
		return out, nil
	}
	var rows []model.Disbursement
	if err := s.db.WithContext(ctx).
		Where("reference_type = ? AND reference_id IN ?", refType, refIDs).
		Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		r := &rows[i]
		if _, seen := out[r.ReferenceID]; !seen { // created_at DESC ⇒ first seen is latest
			out[r.ReferenceID] = recordOf(r)
		}
	}
	return out, nil
}

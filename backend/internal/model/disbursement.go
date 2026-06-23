package model

import "time"

// Disbursement is a provider-agnostic money-out record (a payout to a bank /
// e-wallet). Created in-process by payroll (one per payslip); status advances via
// the gateway webhook. `ExternalID` is our idempotency key; `Provider`/`Status`/
// `ChannelCode` are plain strings so adding a gateway never touches this model.
type Disbursement struct {
	ID            string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Provider      string `gorm:"not null"`
	ExternalID    string `gorm:"uniqueIndex;not null;column:external_id"`
	ReferenceType string `gorm:"not null;default:'';column:reference_type"` // e.g. "payslip"
	ReferenceID   string `gorm:"not null;default:'';column:reference_id"`
	ChannelCode   string `gorm:"not null;default:'';column:channel_code"`
	AccountNumber string `gorm:"not null;default:'';column:account_number"`
	AccountHolder string `gorm:"not null;default:'';column:account_holder"`
	Amount        int64  `gorm:"not null;default:0"`            // minor units
	Currency      string `gorm:"not null;default:'IDR'"`
	Status        string `gorm:"not null;default:'PENDING'"`    // PENDING|PROCESSING|COMPLETED|FAILED|VOIDED
	ProviderRef   string `gorm:"not null;default:'';column:provider_ref"`
	FailureReason string `gorm:"not null;default:'';column:failure_reason"`
	CreatedBy     string `gorm:"not null;type:uuid;column:created_by"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CompletedAt   *time.Time `gorm:"column:completed_at"`
}

func (Disbursement) TableName() string { return "disbursements" }

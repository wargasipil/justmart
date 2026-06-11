package prescription

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	prescriptionifacev1 "github.com/justmart/backend/gen/prescription_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

const (
	rxStatusActive    = "ACTIVE"
	rxStatusDispensed = "DISPENSED"
	rxStatusExpired   = "EXPIRED"
	rxStatusVoided    = "VOIDED"

	defaultRxValidityDays = 90
)

// Exported status constants + computation, so the POS (sale package) can enforce
// Rx coverage using the single source of truth for the Rx status rule.
const (
	StatusActive    = rxStatusActive
	StatusDispensed = rxStatusDispensed
	StatusExpired   = rxStatusExpired
	StatusVoided    = rxStatusVoided
)

// ComputeStatus is the exported wrapper over computeRxStatus for cross-package
// callers (POS Rx enforcement). The DB stores only ACTIVE/VOIDED; the rest are
// derived read-through.
func ComputeStatus(rx *model.Prescription, now time.Time) string {
	return computeRxStatus(rx, now)
}

// lockByID loads a prescription FOR UPDATE (Postgres) so a mutating handler can
// read-check-write atomically. SQLite serializes via the single-writer pool.
func (s *PrescriptionService) lockByID(tx *gorm.DB, id string) (*model.Prescription, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var rx model.Prescription
	err := common.RowLock(tx).Where("id = ?", id).First(&rx).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("prescription not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &rx, nil
}

// loadFull reads a prescription with its items preloaded, for response mapping.
func (s *PrescriptionService) loadFull(ctx context.Context, id string) (*model.Prescription, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var rx model.Prescription
	err := s.db.WithContext(ctx).Preload("Items").Where("id = ?", id).First(&rx).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("prescription not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &rx, nil
}

func parseDateRequired(s, field string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("%s required", field)
	}
	t, err := time.Parse(common.DateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be YYYY-MM-DD: %w", field, err)
	}
	return t, nil
}

func parseDateMaybe(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(common.DateLayout, s)
	if err != nil {
		return nil, fmt.Errorf("must be YYYY-MM-DD: %w", err)
	}
	return &t, nil
}

// assignRxNo increments the per-year counter and returns RX-YYYY-NNNN. Mirrors
// assignSaleNo in the sale package (atomic UPDATE; no explicit row lock needed).
func assignRxNo(tx *gorm.DB, now time.Time) (string, error) {
	year := now.Year()
	var counter model.RxCounter
	err := tx.Where("year = ?", year).First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		counter = model.RxCounter{Year: year, LastSeq: 0}
		if err := tx.Create(&counter).Error; err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	if err := tx.Model(&model.RxCounter{}).
		Where("year = ?", year).
		Update("last_seq", gorm.Expr("last_seq + 1")).Error; err != nil {
		return "", err
	}
	if err := tx.Where("year = ?", year).First(&counter).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("RX-%d-%04d", year, counter.LastSeq), nil
}

// computeRxStatus derives the live status from stored fields. The DB only stores
// ACTIVE / VOIDED; DISPENSED and EXPIRED are computed read-through.
func computeRxStatus(rx *model.Prescription, now time.Time) string {
	if rx.Status == rxStatusVoided {
		return rxStatusVoided
	}
	allDispensed := len(rx.Items) > 0
	for _, it := range rx.Items {
		if it.DispensedQty < it.PrescribedQty {
			allDispensed = false
			break
		}
	}
	if allDispensed {
		return rxStatusDispensed
	}
	// Compare against the day AFTER ExpiresAt so a script issued + expiring the
	// same day stays valid until end-of-day.
	if now.After(rx.ExpiresAt.AddDate(0, 0, 1)) {
		return rxStatusExpired
	}
	return rxStatusActive
}

// ---------- Proto mapping ----------

func rxToProto(rx *model.Prescription, status string) *prescriptionifacev1.Prescription {
	out := &prescriptionifacev1.Prescription{
		Id:         rx.ID,
		CustomerId: rx.CustomerID,
		IssuerName: rx.IssuerName,
		IssuedAt:   rx.IssuedAt.Format(common.DateLayout),
		ExpiresAt:  rx.ExpiresAt.Format(common.DateLayout),
		Note:       rx.Note,
		Status:     status,
		CreatedBy:  rx.CreatedBy,
		CreatedAt:  rx.CreatedAt.Unix(),
	}
	if rx.RxNo != nil {
		out.RxNo = *rx.RxNo
	}
	if rx.BranchID != nil {
		out.BranchId = *rx.BranchID
	}
	for i := range rx.Items {
		out.Items = append(out.Items, rxItemToProto(&rx.Items[i]))
	}
	return out
}

func rxItemToProto(it *model.PrescriptionItem) *prescriptionifacev1.PrescriptionItem {
	return &prescriptionifacev1.PrescriptionItem{
		Id:                 it.ID,
		PrescriptionId:     it.PrescriptionID,
		ProductId:          it.ProductID,
		PrescribedQty:      it.PrescribedQty,
		DispensedQty:       it.DispensedQty,
		DosageInstructions: it.DosageInstructions,
		Note:               it.Note,
	}
}

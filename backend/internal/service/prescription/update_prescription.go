package prescription

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	prescriptionifacev1 "github.com/justmart/backend/gen/prescription_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *PrescriptionService) UpdatePrescription(
	ctx context.Context,
	req *connect.Request[prescriptionifacev1.UpdatePrescriptionRequest],
) (*connect.Response[prescriptionifacev1.UpdatePrescriptionResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rx, err := s.lockByID(tx, req.Msg.Id)
		if err != nil {
			return err
		}
		if rx.Status != rxStatusActive {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("only ACTIVE prescriptions can be edited; this one is %s", rx.Status))
		}
		// Block edits once any dispensing has happened.
		var dispensed int64
		if err := tx.Model(&model.PrescriptionItem{}).
			Where("prescription_id = ? AND dispensed_qty > 0", rx.ID).
			Count(&dispensed).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if dispensed > 0 {
			return common.TokenError(connect.CodeFailedPrecondition, "prescription.already_dispensed")
		}

		issuer := strings.TrimSpace(req.Msg.IssuerName)
		if issuer == "" {
			return common.TokenError(connect.CodeInvalidArgument, "prescription.issuer_required")
		}
		issued, err := parseDateRequired(req.Msg.IssuedAt, "issued_at")
		if err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		}
		expires, err := parseDateMaybe(req.Msg.ExpiresAt)
		if err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		}
		if expires == nil {
			d := issued.AddDate(0, 0, defaultRxValidityDays)
			expires = &d
		}
		if expires.Before(issued) {
			return common.TokenError(connect.CodeInvalidArgument, "prescription.expires_before_issued")
		}
		if req.Msg.BiayaJasa < 0 {
			return common.TokenError(connect.CodeInvalidArgument, "prescription.fee_negative")
		}

		updates := map[string]any{
			"issuer_name":     issuer,
			"issued_at":       issued,
			"expires_at":      *expires,
			"note":            strings.TrimSpace(req.Msg.Note),
			"biaya_jasa":      req.Msg.BiayaJasa,
			"patient_age":     req.Msg.PatientAge,
			"patient_weight":  strings.TrimSpace(req.Msg.PatientWeight),
			"patient_allergy": strings.TrimSpace(req.Msg.PatientAllergy),
		}
		if err := tx.Model(rx).Updates(updates).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		if len(req.Msg.Items) > 0 {
			if err := tx.Where("prescription_id = ?", rx.ID).Delete(&model.PrescriptionItem{}).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			items := make([]model.PrescriptionItem, 0, len(req.Msg.Items))
			for _, in := range req.Msg.Items {
				if in.PrescribedQty <= 0 {
					return common.TokenError(connect.CodeInvalidArgument, "prescription.qty_invalid")
				}
				if strings.TrimSpace(in.ProductId) == "" {
					return common.TokenError(connect.CodeInvalidArgument, "prescription.item_product_required")
				}
				items = append(items, model.PrescriptionItem{
					PrescriptionID:     rx.ID,
					ProductID:          in.ProductId,
					PrescribedQty:      in.PrescribedQty,
					DispensedQty:       0,
					DosageInstructions: strings.TrimSpace(in.DosageInstructions),
					Note:               strings.TrimSpace(in.Note),
				})
			}
			if err := tx.Create(&items).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	full, err := s.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	status := computeRxStatus(full, time.Now())
	return connect.NewResponse(&prescriptionifacev1.UpdatePrescriptionResponse{
		Prescription: rxToProto(full, status),
	}), nil
}

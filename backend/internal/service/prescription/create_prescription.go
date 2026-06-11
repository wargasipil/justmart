package prescription

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	prescriptionifacev1 "github.com/justmart/backend/gen/prescription_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *PrescriptionService) CreatePrescription(
	ctx context.Context,
	req *connect.Request[prescriptionifacev1.CreatePrescriptionRequest],
) (*connect.Response[prescriptionifacev1.CreatePrescriptionResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.CustomerId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("customer_id required"))
	}
	if strings.TrimSpace(req.Msg.IssuerName) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("issuer_name required"))
	}
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item required"))
	}

	issued, err := parseDateRequired(req.Msg.IssuedAt, "issued_at")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	expires, err := parseDateMaybe(req.Msg.ExpiresAt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if expires == nil {
		d := issued.AddDate(0, 0, defaultRxValidityDays)
		expires = &d
	}
	if expires.Before(issued) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("expires_at must be on/after issued_at"))
	}
	if req.Msg.BiayaJasa < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("biaya_jasa must be >= 0"))
	}

	var rx model.Prescription
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rx = model.Prescription{
			CustomerID:     req.Msg.CustomerId,
			IssuerName:     strings.TrimSpace(req.Msg.IssuerName),
			IssuedAt:       issued,
			ExpiresAt:      *expires,
			Note:           strings.TrimSpace(req.Msg.Note),
			Status:         rxStatusActive,
			CreatedBy:      caller.UserID,
			BiayaJasa:      req.Msg.BiayaJasa,
			PatientAge:     req.Msg.PatientAge,
			PatientWeight:  strings.TrimSpace(req.Msg.PatientWeight),
			PatientAllergy: strings.TrimSpace(req.Msg.PatientAllergy),
		}
		rxNo, err := assignRxNo(tx, time.Now())
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		rx.RxNo = &rxNo
		if err := tx.Create(&rx).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		items := make([]model.PrescriptionItem, 0, len(req.Msg.Items))
		for _, in := range req.Msg.Items {
			if in.PrescribedQty <= 0 {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("prescribed_qty must be > 0"))
			}
			if strings.TrimSpace(in.ProductId) == "" {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("product_id required on each item"))
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
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	full, err := s.loadFull(ctx, rx.ID)
	if err != nil {
		return nil, err
	}
	status := computeRxStatus(full, time.Now())
	return connect.NewResponse(&prescriptionifacev1.CreatePrescriptionResponse{
		Prescription: rxToProto(full, status),
	}), nil
}

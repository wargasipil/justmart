package prescription

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	prescriptionifacev1 "github.com/justmart/backend/gen/prescription_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

func (s *PrescriptionService) VoidPrescription(
	ctx context.Context,
	req *connect.Request[prescriptionifacev1.VoidPrescriptionRequest],
) (*connect.Response[prescriptionifacev1.VoidPrescriptionResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rx, err := s.lockByID(tx, req.Msg.Id)
		if err != nil {
			return err
		}
		if rx.Status == rxStatusVoided {
			return nil // idempotent
		}
		return tx.Model(rx).Update("status", rxStatusVoided).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	full, err := s.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	status := computeRxStatus(full, time.Now())
	return connect.NewResponse(&prescriptionifacev1.VoidPrescriptionResponse{
		Prescription: rxToProto(full, status),
	}), nil
}

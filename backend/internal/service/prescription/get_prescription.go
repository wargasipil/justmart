package prescription

import (
	"context"
	"time"

	"connectrpc.com/connect"

	prescriptionifacev1 "github.com/justmart/backend/gen/prescription_iface/v1"
)

func (s *PrescriptionService) GetPrescription(
	ctx context.Context,
	req *connect.Request[prescriptionifacev1.GetPrescriptionRequest],
) (*connect.Response[prescriptionifacev1.GetPrescriptionResponse], error) {
	rx, err := s.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	status := computeRxStatus(rx, time.Now())
	return connect.NewResponse(&prescriptionifacev1.GetPrescriptionResponse{
		Prescription: rxToProto(rx, status),
	}), nil
}

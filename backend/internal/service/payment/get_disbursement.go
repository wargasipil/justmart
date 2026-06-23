package payment

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	paymentifacev1 "github.com/justmart/backend/gen/payment_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (s *Service) GetDisbursement(
	ctx context.Context,
	req *connect.Request[paymentifacev1.GetDisbursementRequest],
) (*connect.Response[paymentifacev1.GetDisbursementResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var d model.Disbursement
	err := s.db.WithContext(ctx).Where("id = ?", req.Msg.Id).First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("disbursement not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&paymentifacev1.GetDisbursementResponse{Disbursement: disbursementToProto(&d)}), nil
}

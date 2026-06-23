package payment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	paymentifacev1 "github.com/justmart/backend/gen/payment_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// RetryDisbursement re-submits a FAILED disbursement under a fresh external_id
// (so the gateway treats it as a new attempt, not a deduped repeat).
func (s *Service) RetryDisbursement(
	ctx context.Context,
	req *connect.Request[paymentifacev1.RetryDisbursementRequest],
) (*connect.Response[paymentifacev1.RetryDisbursementResponse], error) {
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
	if d.Status != StatusFailed {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "payment.retry_not_failed")
	}
	prov, ok := s.providerByName(d.Provider)
	if !ok {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "payment.no_provider")
	}

	d.ExternalID = fmt.Sprintf("%s_r%d", externalID(d.ReferenceType, d.ReferenceID), time.Now().UnixNano())
	d.Status = StatusPending
	d.FailureReason = ""
	d.ProviderRef = ""
	if err := s.db.WithContext(ctx).Model(&model.Disbursement{}).Where("id = ?", d.ID).
		Updates(map[string]any{
			"external_id":    d.ExternalID,
			"status":         StatusPending,
			"failure_reason": "",
			"provider_ref":   "",
		}).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	s.submit(ctx, prov, &d) // outside any tx

	var full model.Disbursement
	if err := s.db.WithContext(ctx).Where("id = ?", d.ID).First(&full).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&paymentifacev1.RetryDisbursementResponse{Disbursement: disbursementToProto(&full)}), nil
}

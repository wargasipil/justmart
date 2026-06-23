package payment

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	paymentifacev1 "github.com/justmart/backend/gen/payment_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *Service) ListDisbursements(
	ctx context.Context,
	req *connect.Request[paymentifacev1.ListDisbursementsRequest],
) (*connect.Response[paymentifacev1.ListDisbursementsResponse], error) {
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	status := strings.ToUpper(strings.TrimSpace(req.Msg.Status))
	applyFilters := func(q *gorm.DB) *gorm.DB {
		if status != "" {
			q = q.Where("status = ?", status)
		}
		if rt := strings.TrimSpace(req.Msg.ReferenceType); rt != "" {
			q = q.Where("reference_type = ?", rt)
		}
		if rid := strings.TrimSpace(req.Msg.ReferenceId); rid != "" {
			q = q.Where("reference_id = ?", rid)
		}
		return q
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Disbursement{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.Disbursement
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Disbursement{})).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*paymentifacev1.Disbursement, 0, len(rows))
	for i := range rows {
		out = append(out, disbursementToProto(&rows[i]))
	}
	return connect.NewResponse(&paymentifacev1.ListDisbursementsResponse{
		Disbursements: out,
		Total:         int32(total),
	}), nil
}

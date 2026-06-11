package prescription

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	prescriptionifacev1 "github.com/justmart/backend/gen/prescription_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *PrescriptionService) ListPrescriptions(
	ctx context.Context,
	req *connect.Request[prescriptionifacev1.ListPrescriptionsRequest],
) (*connect.Response[prescriptionifacev1.ListPrescriptionsResponse], error) {
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		if req.Msg.CustomerId != "" {
			q = q.Where("customer_id = ?", req.Msg.CustomerId)
		}
		return q
	}

	// total reflects the base rows (customer scope); the computed-status filter
	// below is a client-side refinement over the fetched page only. Documented
	// v1 limitation, mirrors ListPrescriptions in the original Phase 6.
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Prescription{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var rows []model.Prescription
	if err := applyFilters(s.db.WithContext(ctx).Preload("Items")).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	filter := strings.TrimSpace(strings.ToUpper(req.Msg.Status))
	now := time.Now()
	out := make([]*prescriptionifacev1.Prescription, 0, len(rows))
	for i := range rows {
		status := computeRxStatus(&rows[i], now)
		if filter != "" && status != filter {
			continue
		}
		out = append(out, rxToProto(&rows[i], status))
	}
	return connect.NewResponse(&prescriptionifacev1.ListPrescriptionsResponse{
		Prescriptions: out,
		Total:         int32(total),
	}), nil
}

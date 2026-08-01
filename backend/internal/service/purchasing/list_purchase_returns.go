package purchasing

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (p *PurchaseReturns) ListPurchaseReturns(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.ListPurchaseReturnsRequest],
) (*connect.Response[purchasingifacev1.ListPurchaseReturnsResponse], error) {
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)

	// One filter closure feeds both the count and the page, so the two can't
	// drift. Preload("Items") is on the PAGE query only — counting with a
	// preload would load every child row just to throw them away.
	applyFilters := func(q *gorm.DB) *gorm.DB {
		q = q.Model(&model.PurchaseReturn{})
		if req.Msg.PurchaseOrderId != "" {
			q = q.Where("purchase_order_id = ?", req.Msg.PurchaseOrderId)
		}
		return q
	}
	var total int64
	if err := applyFilters(p.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.PurchaseReturn
	if err := applyFilters(p.db.WithContext(ctx)).
		Preload("Items").
		Order("created_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*purchasingifacev1.PurchaseReturn, 0, len(rows))
	for i := range rows {
		out = append(out, purchaseReturnToProto(&rows[i]))
	}
	return connect.NewResponse(&purchasingifacev1.ListPurchaseReturnsResponse{
		Returns: out,
		Total:   int32(total),
	}), nil
}

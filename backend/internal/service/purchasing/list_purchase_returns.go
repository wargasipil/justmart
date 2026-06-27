package purchasing

import (
	"context"

	"connectrpc.com/connect"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (p *PurchaseReturns) ListPurchaseReturns(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.ListPurchaseReturnsRequest],
) (*connect.Response[purchasingifacev1.ListPurchaseReturnsResponse], error) {
	q := p.db.WithContext(ctx).Preload("Items").Order("created_at DESC")
	if req.Msg.PurchaseOrderId != "" {
		q = q.Where("purchase_order_id = ?", req.Msg.PurchaseOrderId)
	}
	var rows []model.PurchaseReturn
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*purchasingifacev1.PurchaseReturn, 0, len(rows))
	for i := range rows {
		out = append(out, purchaseReturnToProto(&rows[i]))
	}
	return connect.NewResponse(&purchasingifacev1.ListPurchaseReturnsResponse{Returns: out}), nil
}

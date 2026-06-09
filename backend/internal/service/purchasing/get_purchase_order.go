package purchasing

import (
	"context"

	"connectrpc.com/connect"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func (p *PurchaseOrders) GetPurchaseOrder(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.GetPurchaseOrderRequest],
) (*connect.Response[purchasingifacev1.GetPurchaseOrderResponse], error) {
	po, err := p.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.GetPurchaseOrderResponse{Order: poToProto(po)}), nil
}

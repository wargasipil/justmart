package purchasing

import (
	"context"

	"connectrpc.com/connect"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func (p *PurchaseReceipts) GetReceipt(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.GetReceiptRequest],
) (*connect.Response[purchasingifacev1.GetReceiptResponse], error) {
	r, err := p.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.GetReceiptResponse{Receipt: receiptToProto(r)}), nil
}

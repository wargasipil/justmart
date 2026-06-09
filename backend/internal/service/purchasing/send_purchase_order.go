package purchasing

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

func (p *PurchaseOrders) SendPurchaseOrder(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.SendPurchaseOrderRequest],
) (*connect.Response[purchasingifacev1.SendPurchaseOrderResponse], error) {
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		po, err := p.lockByID(tx, req.Msg.Id)
		if err != nil {
			return err
		}
		if po.Status != poStatusDraft {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("only DRAFT POs can be sent; this one is %s", po.Status))
		}
		now := time.Now()
		return tx.Model(po).Updates(map[string]any{
			"status":  poStatusSent,
			"sent_at": now,
		}).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	full, err := p.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.SendPurchaseOrderResponse{Order: poToProto(full)}), nil
}

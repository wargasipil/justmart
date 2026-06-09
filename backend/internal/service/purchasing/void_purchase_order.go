package purchasing

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

func (p *PurchaseOrders) VoidPurchaseOrder(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.VoidPurchaseOrderRequest],
) (*connect.Response[purchasingifacev1.VoidPurchaseOrderResponse], error) {
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		po, err := p.lockByID(tx, req.Msg.Id)
		if err != nil {
			return err
		}
		if po.Status != poStatusDraft && po.Status != poStatusSent {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("only DRAFT or SENT POs can be voided; this one is %s", po.Status))
		}
		return tx.Model(po).Update("status", poStatusVoided).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	full, err := p.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.VoidPurchaseOrderResponse{Order: poToProto(full)}), nil
}

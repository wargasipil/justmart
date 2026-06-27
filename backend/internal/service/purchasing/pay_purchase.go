package purchasing

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

func (p *PurchasePayments) PayPurchase(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.PayPurchaseRequest],
) (*connect.Response[purchasingifacev1.PayPurchaseResponse], error) {
	if req.Msg.PurchaseOrderId == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.po_required")
	}
	if req.Msg.Amount <= 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.amount_invalid")
	}

	var paid, outstanding int64

	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ordersSvc := &PurchaseOrders{db: tx}
		po, err := ordersSvc.lockByID(tx, req.Msg.PurchaseOrderId)
		if err != nil {
			return err
		}
		if po.Status == poStatusVoided {
			return common.TokenError(connect.CodeFailedPrecondition, "purchasing.po_voided")
		}
		newPaid := po.PaidAmount + req.Msg.Amount
		if err := tx.Model(po).Update("paid_amount", newPaid).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		po.PaidAmount = newPaid
		// CLOSED auto-transition if fully paid and fully received.
		if err := maybeCloseIfPaid(tx, po); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		paid = po.PaidAmount
		outstanding = po.OrderedTotal - po.PaidAmount - po.ReturnedAmount
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	return connect.NewResponse(&purchasingifacev1.PayPurchaseResponse{
		PurchaseOrderId: req.Msg.PurchaseOrderId,
		PaidAmount:      paid,
		Outstanding:     outstanding,
	}), nil
}

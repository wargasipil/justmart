package purchasing

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// Whether a restock order can still be edited (UpdatePurchaseOrder) is ONE
// rule with two consumers: the handler, which refuses an edit, and the
// PurchaseOrder.editable flag every order response carries, which the UI gates
// its Edit action on. Both go through editRefusal, so they cannot drift.
//
// The rule: a DRAFT is always editable. A SENT order is editable only while it
// has NO purchase_receipts row at all. A CANCELLED receipt still counts: its
// purchase_receipt_items keep their purchase_order_item_id FK, and an edit
// deletes and re-creates every order line, so the delete would hit that FK
// (and purchase returns hang off receipts too). Anything past SENT has goods
// in the warehouse and is never editable here.
//
// Editing a SENT order keeps it SENT (status and sent_at are untouched); the
// change is not sent to the supplier, which the edit page warns about.

const (
	tokenNotEditable     = "purchasing.not_editable"
	tokenEditHasReceipts = "purchasing.edit_has_receipts"
)

// editRefusal returns the stable token UpdatePurchaseOrder refuses with for an
// order in this status, or the empty string when the edit is allowed.
// hasReceipts is only consulted for a SENT order.
func editRefusal(status string, hasReceipts bool) string {
	switch status {
	case poStatusDraft:
		return ""
	case poStatusSent:
		if hasReceipts {
			return tokenEditHasReceipts
		}
		return ""
	default:
		return tokenNotEditable
	}
}

// editableStatus is editRefusal as a yes/no, for the proto flag.
func editableStatus(status string, hasReceipts bool) bool {
	return editRefusal(status, hasReceipts) == ""
}

// ordersWithReceipts reports which of the given orders have ever had a
// receipt, cancelled ones included. One query for any number of ids.
func ordersWithReceipts(db *gorm.DB, poIDs []string) (map[string]bool, error) {
	out := make(map[string]bool, len(poIDs))
	if len(poIDs) == 0 {
		return out, nil
	}
	var ids []string
	if err := db.Model(&model.PurchaseReceipt{}).
		Distinct("purchase_order_id").
		Where("purchase_order_id IN ?", poIDs).
		Pluck("purchase_order_id", &ids).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

// checkOrderEditable is UpdatePurchaseOrder's guard. Call it inside the edit's
// tx, after lockByID, so a receipt cannot land between the check and the line
// replace (CreateReceipt locks the same order row first).
func checkOrderEditable(tx *gorm.DB, po *model.PurchaseOrder) error {
	hasReceipts := false
	// Only a SENT order's answer depends on receipts; skip the query otherwise.
	if po.Status == poStatusSent {
		has, err := ordersWithReceipts(tx, []string{po.ID})
		if err != nil {
			return err
		}
		hasReceipts = has[po.ID]
	}
	if token := editRefusal(po.Status, hasReceipts); token != "" {
		return common.TokenError(connect.CodeFailedPrecondition, token)
	}
	return nil
}

// orderToProto converts one loaded order for a single-order response (Get and
// the write RPCs) with its editable flag filled.
func (p *PurchaseOrders) orderToProto(ctx context.Context, po *model.PurchaseOrder) (*purchasingifacev1.PurchaseOrder, error) {
	out := poToProto(po)
	if err := markEditable(p.db.WithContext(ctx), out); err != nil {
		return nil, err
	}
	return out, nil
}

// markEditable fills PurchaseOrder.editable on already-converted orders, with
// one receipts query for the whole set (no N+1 on a list page). Every RPC that
// returns an order calls it, so the flag is never left at its false default.
func markEditable(db *gorm.DB, orders ...*purchasingifacev1.PurchaseOrder) error {
	sentIDs := make([]string, 0, len(orders))
	for _, o := range orders {
		if o.Status == purchasingifacev1.POStatus_PO_STATUS_SENT {
			sentIDs = append(sentIDs, o.Id)
		}
	}
	has, err := ordersWithReceipts(db, sentIDs)
	if err != nil {
		return err
	}
	for _, o := range orders {
		o.Editable = editableStatus(poStatusToString(o.Status), has[o.Id])
	}
	return nil
}

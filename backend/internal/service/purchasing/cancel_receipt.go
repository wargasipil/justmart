package purchasing

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// Why a cancel is refused. Stable tokens: returned as the error on CancelReceipt
// AND precomputed onto PurchaseReceipt.cancel_blocked_reason by
// enrichReturnable, so the UI disables the button with the same wording it would
// have failed with. Keep the two in sync — the enrich is a batched read of the
// same conditions the handler re-checks under lock.
const (
	// The lot has been touched since it arrived: sold, transferred, adjusted,
	// written off, counted, or partly returned. Cancelling would have to invent
	// stock history; that is what a purchase RETURN is for.
	cancelBlockedConsumed = "purchasing.receipt_lot_consumed"
	// An open stocktake session already lists the lot. Deleting the batch would
	// break that session's line (FK), and silently deleting someone's in-progress
	// count is worse than asking them to drop the line first.
	cancelBlockedStocktake = "purchasing.receipt_lot_in_stocktake"
	// The PO is settled. recomputePOStatus deliberately never reopens a CLOSED
	// PO, so a cancel here would leave it settled with a lowered received_qty.
	cancelBlockedPOClosed = "purchasing.receipt_cancel_po_closed"
	// Already cancelled — nothing left to undo.
	cancelBlockedVoided = "purchasing.receipt_already_voided"
)

// CancelReceipt undoes an accepted restock that was entered in error ("batal
// terima"), as opposed to CreatePurchaseReturn which records goods physically
// going back to the supplier and accrues a refund.
//
// It is permitted only while every lot the receipt created is still UNTOUCHED —
// exactly one stock movement, the PURCHASE that created it, for the full
// received qty. Given that, the lot and its movement are DELETED rather than
// reversed with a compensating movement: nothing physically happened, so an
// arrived-then-left pair would record a fiction, and the zero-stock ghost lot it
// leaves behind would surface in batch pickers and every future stocktake
// forever. Deleting a provably-untouched lot under a row lock does not weaken
// the ledger invariant (stock = SUM(movements), never mutated under a reader) —
// the lot simply ceases to exist.
//
// The receipt DOCUMENT survives, marked voided, because its RCV-YYYY-NNNN is
// already burned from the counter and an unexplained gap in that sequence is
// exactly what an audit questions.
func (p *PurchaseReceipts) CancelReceipt(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.CancelReceiptRequest],
) (*connect.Response[purchasingifacev1.CancelReceiptResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Msg.Id) == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.receipt_required")
	}
	reason := strings.TrimSpace(req.Msg.Reason)
	if reason == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.reason_required")
	}
	receiptID := strings.TrimSpace(req.Msg.Id)

	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var r model.PurchaseReceipt
		if e := tx.Preload("Items").Where("id = ?", receiptID).First(&r).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("receipt not found"))
			}
			return connect.NewError(connect.CodeInternal, e)
		}
		if r.VoidedAt != nil {
			return common.TokenError(connect.CodeFailedPrecondition, cancelBlockedVoided)
		}

		// Lock the PO: received_qty and status are about to move.
		po, e := (&PurchaseOrders{db: tx}).lockByID(tx, r.PurchaseOrderID)
		if e != nil {
			return e
		}
		if po.Status == poStatusClosed {
			return common.TokenError(connect.CodeFailedPrecondition, cancelBlockedPOClosed)
		}

		// Lock every lot first, in one deterministic-order statement, so a
		// concurrent sale can't slip a movement in between the check and the
		// delete (LockBatchesByID orders by id — same discipline as CompleteSale).
		batchIDs := make([]string, 0, len(r.Items))
		for i := range r.Items {
			if r.Items[i].BatchID != nil {
				batchIDs = append(batchIDs, *r.Items[i].BatchID)
			}
		}
		if len(batchIDs) > 0 {
			if e := common.LockBatchesByID(tx, batchIDs); e != nil {
				return connect.NewError(connect.CodeInternal, e)
			}
		}

		// restockKey identifies a product_last_restocks row; collected here and
		// recomputed once per distinct key after the lines are undone (two lines
		// of the same product on one receipt share a key).
		type restockKey struct{ warehouseID, productID, supplierID string }
		touched := map[restockKey]struct{}{}

		for i := range r.Items {
			item := &r.Items[i]

			if item.BatchID != nil {
				// The untouched assertion. Exactly one movement, and it must be
				// the PURCHASE this receipt wrote for the full qty — anything
				// else (a sale, transfer, adjustment, write-off, stocktake
				// variance, partial return) means stock history depends on this
				// lot and only a return can unwind it.
				var mvs []model.StockMovement
				if e := tx.Where("batch_id = ?", *item.BatchID).Find(&mvs).Error; e != nil {
					return connect.NewError(connect.CodeInternal, e)
				}
				if len(mvs) != 1 || mvs[0].Type != "PURCHASE" || mvs[0].Qty != item.Qty {
					return common.TokenError(connect.CodeFailedPrecondition, cancelBlockedConsumed)
				}

				// stocktake_lines.batch_id is a NOT NULL FK, so an open session
				// holding this lot would turn the delete below into an opaque
				// constraint error. Refuse with a reason the operator can act on.
				var stocktakeLines int64
				if e := tx.Model(&model.StocktakeLine{}).
					Where("batch_id = ?", *item.BatchID).Count(&stocktakeLines).Error; e != nil {
					return connect.NewError(connect.CodeInternal, e)
				}
				if stocktakeLines > 0 {
					return common.TokenError(connect.CodeFailedPrecondition, cancelBlockedStocktake)
				}

				// Drop the receipt line's reference before the batch row goes, so
				// the FK never sees a dangling link.
				if e := tx.Model(&model.PurchaseReceiptItem{}).
					Where("id = ?", item.ID).
					Update("batch_id", nil).Error; e != nil {
					return connect.NewError(connect.CodeInternal, e)
				}
				if e := tx.Where("id = ?", mvs[0].ID).
					Delete(&model.StockMovement{}).Error; e != nil {
					return connect.NewError(connect.CodeInternal, e)
				}
				if e := tx.Where("id = ?", *item.BatchID).
					Delete(&model.Batch{}).Error; e != nil {
					return connect.NewError(connect.CodeInternal, e)
				}
			}

			// Give the ordered qty back so the PO reads as still awaiting it.
			if e := tx.Model(&model.PurchaseOrderItem{}).
				Where("id = ?", item.PurchaseOrderItemID).
				Update("received_qty", gorm.Expr("received_qty - ?", item.Qty)).Error; e != nil {
				return connect.NewError(connect.CodeInternal, e)
			}

			touched[restockKey{po.WarehouseID, item.ProductID, po.SupplierID}] = struct{}{}
		}

		// Undo the restock records. The log row is found by receipt_id; the
		// last-value row must be REBUILT from the newest survivor, not reverted —
		// it is an upsert, so it holds no memory of what it replaced.
		if e := tx.Where("receipt_id = ?", r.ID).
			Delete(&model.ProductRestockLog{}).Error; e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}
		for k := range touched {
			if e := rebuildLastRestock(tx, k.warehouseID, k.productID, k.supplierID); e != nil {
				return e
			}
		}

		now := time.Now()
		if e := tx.Model(&model.PurchaseReceipt{}).Where("id = ?", r.ID).
			Updates(map[string]any{
				"voided_at":   now,
				"voided_by":   caller.UserID,
				"void_reason": reason,
			}).Error; e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}

		// Reflect the reduced received_qty: RECEIVED → PARTIALLY_RECEIVED, or
		// back to SENT when this was the only receipt.
		if e := recomputePOStatus(tx, po); e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	full, err := p.loadFull(ctx, receiptID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.CancelReceiptResponse{
		Receipt: receiptToProto(full),
	}), nil
}

// rebuildLastRestock restores product_last_restocks for one (warehouse,
// product, supplier) from the newest surviving log row, or deletes it when the
// cancelled receipt was the only restock on record.
//
// This is the subtle half of a cancel: recordRestock UPSERTS the last value, so
// the row carries no history of what it overwrote. Reverting it in place would
// leave the product showing the cancelled receipt's cost forever.
func rebuildLastRestock(tx *gorm.DB, warehouseID, productID, supplierID string) error {
	var prev model.ProductRestockLog
	err := tx.Where("warehouse_id = ? AND product_id = ? AND supplier_id = ?",
		warehouseID, productID, supplierID).
		Order("restock_arrived_at DESC, created_at DESC").
		First(&prev).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Nothing left — this supplier never (still) restocked this product here.
		if e := tx.Where("warehouse_id = ? AND product_id = ? AND supplier_id = ?",
			warehouseID, productID, supplierID).
			Delete(&model.ProductLastRestock{}).Error; e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}
		return nil
	}
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	if e := tx.Model(&model.ProductLastRestock{}).
		Where("warehouse_id = ? AND product_id = ? AND supplier_id = ?",
			warehouseID, productID, supplierID).
		Updates(map[string]any{
			"last_price":             prev.Price,
			"last_qty":               prev.Qty,
			"last_discount_type":     prev.DiscountType,
			"last_discount_value":    prev.DiscountValue,
			"last_discount_per_item": prev.DiscountPerItem,
			"last_created_at":        prev.RestockCreatedAt,
			"last_arrived_at":        prev.RestockArrivedAt,
			"updated_at":             prev.RestockArrivedAt,
		}).Error; e != nil {
		return connect.NewError(connect.CodeInternal, e)
	}
	return nil
}

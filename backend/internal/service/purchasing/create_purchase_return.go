package purchasing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// CreatePurchaseReturn records a partial "retur pembelian": received goods sent
// back to the supplier. Per line it reverses a receipt line (one batch) with a
// negative PURCHASE_RETURN stock movement, decrements the PO item's received_qty,
// and accrues the refunded value. The PO outstanding then drops by the total
// (and may go negative = a supplier credit); a non-CLOSED PO reopens to
// PARTIALLY_RECEIVED / SENT via recomputePOStatus.
func (p *PurchaseReturns) CreatePurchaseReturn(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.CreatePurchaseReturnRequest],
) (*connect.Response[purchasingifacev1.CreatePurchaseReturnResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.PurchaseOrderId == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.po_required")
	}
	if len(req.Msg.Lines) == 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.lines_required")
	}
	if strings.TrimSpace(req.Msg.Reason) == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.reason_required")
	}
	returnedAt := time.Now()
	if s := strings.TrimSpace(req.Msg.ReturnedAt); s != "" {
		t, perr := time.Parse(common.DateLayout, s)
		if perr != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("returned_at must be YYYY-MM-DD: %w", perr))
		}
		returnedAt = t
	}

	var ret model.PurchaseReturn
	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock + load the PO; only POs with received goods are returnable.
		po, err := (&PurchaseOrders{db: tx}).lockByID(tx, req.Msg.PurchaseOrderId)
		if err != nil {
			return err
		}
		switch po.Status {
		case poStatusPartiallyReceived, poStatusReceived, poStatusClosed:
			// returnable
		default:
			return common.TokenError(connect.CodeFailedPrecondition, "purchasing.po_not_returnable")
		}

		returnNo, err := assignReturnNo(tx, time.Now())
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		ret = model.PurchaseReturn{
			ReturnNo:        &returnNo,
			PurchaseOrderID: po.ID,
			WarehouseID:     po.WarehouseID,
			ReturnedAt:      returnedAt,
			ReturnedBy:      caller.UserID,
			Reason:          strings.TrimSpace(req.Msg.Reason),
			Note:            strings.TrimSpace(req.Msg.Note),
		}
		if err := tx.Create(&ret).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		var totalRefund int64
		seen := map[string]bool{}
		for _, line := range req.Msg.Lines {
			if line.Qty <= 0 {
				return common.TokenError(connect.CodeInvalidArgument, "purchasing.qty_invalid")
			}
			if seen[line.PurchaseReceiptItemId] {
				return common.TokenError(connect.CodeInvalidArgument, "purchasing.duplicate_line")
			}
			seen[line.PurchaseReceiptItemId] = true

			// Load the receipt item and confirm it belongs to this PO.
			var ri model.PurchaseReceiptItem
			if e := tx.Where("id = ?", line.PurchaseReceiptItemId).First(&ri).Error; e != nil {
				if errors.Is(e, gorm.ErrRecordNotFound) {
					return common.TokenError(connect.CodeInvalidArgument, "purchasing.receipt_item_not_found")
				}
				return connect.NewError(connect.CodeInternal, e)
			}
			var rcpt model.PurchaseReceipt
			if e := tx.Select("purchase_order_id").Where("id = ?", ri.PurchaseReceiptID).First(&rcpt).Error; e != nil {
				return connect.NewError(connect.CodeInternal, e)
			}
			if rcpt.PurchaseOrderID != po.ID {
				return common.TokenError(connect.CodeInvalidArgument, "purchasing.receipt_item_not_found")
			}
			if ri.BatchID == nil {
				return common.TokenError(connect.CodeFailedPrecondition, "purchasing.no_batch")
			}
			factor := ri.UnitFactor
			if factor < 1 {
				factor = 1
			}
			baseQty := line.Qty * int32(factor)

			// Lock the batch + assert current on-hand covers it (can't return
			// units already sold / transferred away — "not spent in order").
			if e := common.LockBatchesByID(tx, []string{*ri.BatchID}); e != nil {
				return connect.NewError(connect.CodeInternal, e)
			}
			onHand, e := common.BatchQtyInWarehouse(ctx, tx, *ri.BatchID, po.WarehouseID)
			if e != nil {
				return connect.NewError(connect.CodeInternal, e)
			}
			if int64(baseQty) > onHand {
				return common.TokenError(connect.CodeFailedPrecondition, "purchasing.return_exceeds_on_hand")
			}

			// Negative PURCHASE_RETURN movement (goods leave to the supplier).
			if e := tx.Create(&model.StockMovement{
				BatchID:     *ri.BatchID,
				Qty:         -baseQty,
				Type:        movementTypePurchaseReturn,
				Reason:      fmt.Sprintf("PO %s return %s", common.Deref(po.PoNo), returnNo),
				UserID:      caller.UserID,
				WarehouseID: po.WarehouseID,
			}).Error; e != nil {
				return connect.NewError(connect.CodeInternal, e)
			}

			lineRefund := int64(baseQty) * ri.UnitCostPrice
			totalRefund += lineRefund
			if e := tx.Create(&model.PurchaseReturnItem{
				PurchaseReturnID:      ret.ID,
				PurchaseReceiptItemID: ri.ID,
				PurchaseOrderItemID:   ri.PurchaseOrderItemID,
				ProductID:             ri.ProductID,
				BatchID:               *ri.BatchID,
				Qty:                   baseQty,
				UnitCostPrice:         ri.UnitCostPrice,
				UnitName:              ri.UnitName,
				UnitFactor:            factor,
			}).Error; e != nil {
				return connect.NewError(connect.CodeInternal, e)
			}

			// Net received drops by the returned base qty.
			if e := tx.Model(&model.PurchaseOrderItem{}).
				Where("id = ?", ri.PurchaseOrderItemID).
				Update("received_qty", gorm.Expr("received_qty - ?", baseQty)).Error; e != nil {
				return connect.NewError(connect.CodeInternal, e)
			}
		}

		ret.RefundAmount = totalRefund
		if e := tx.Model(&ret).Update("refund_amount", totalRefund).Error; e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}
		if e := tx.Model(&model.PurchaseOrder{}).Where("id = ?", po.ID).
			Update("returned_amount", gorm.Expr("returned_amount + ?", totalRefund)).Error; e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}

		// Reopen a non-CLOSED PO to reflect the reduced received_qty; CLOSED stays
		// CLOSED (settled) and just carries the supplier credit.
		if e := recomputePOStatus(tx, po); e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	full, err := p.loadFull(ctx, ret.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.CreatePurchaseReturnResponse{
		PurchaseReturn: purchaseReturnToProto(full),
	}), nil
}

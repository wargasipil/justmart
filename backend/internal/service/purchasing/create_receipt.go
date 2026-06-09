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

func (p *PurchaseReceipts) CreateReceipt(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.CreateReceiptRequest],
) (*connect.Response[purchasingifacev1.CreateReceiptResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.PurchaseOrderId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("purchase_order_id required"))
	}
	if len(req.Msg.Lines) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one line required"))
	}

	// Stock lands in the PO's warehouse (stamped at CreatePurchaseOrder time),
	// not the caller's active warehouse. Loaded inside the tx below.
	var warehouseID string

	// Default receipt date to today.
	receivedAt := time.Now()
	if req.Msg.ReceivedAt != "" {
		t, err := time.Parse("2006-01-02", req.Msg.ReceivedAt)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("received_at must be YYYY-MM-DD: %w", err))
		}
		receivedAt = t
	}

	var receipt model.PurchaseReceipt

	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the PO; reject if not in a receivable state.
		var po model.PurchaseOrder
		if err := tx.Where("id = ?", req.Msg.PurchaseOrderId).First(&po).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("purchase order not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		switch po.Status {
		case poStatusSent, poStatusPartiallyReceived:
			// receivable — proceed
		default:
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("cannot receive a PO in status %s; send it first", po.Status))
		}
		// Pin stock destination to the PO's warehouse.
		warehouseID = po.WarehouseID

		// Create the receipt header with an assigned receipt_no.
		receiptNo, err := assignReceiptNo(tx, time.Now())
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		// Faktur is captured once at PO creation; the receive flow no longer asks
		// for it. Inherit the PO's invoice_no when the request omits one (an
		// explicit value still wins — e.g. a partial delivery with its own faktur
		// sent via the API).
		invoiceNo := strings.TrimSpace(req.Msg.InvoiceNo)
		if invoiceNo == "" {
			invoiceNo = po.InvoiceNo
		}
		receipt = model.PurchaseReceipt{
			ReceiptNo:       &receiptNo,
			PurchaseOrderID: po.ID,
			ReceivedAt:      receivedAt,
			ReceivedBy:      caller.UserID,
			Note:            strings.TrimSpace(req.Msg.Note),
			InvoiceNo:       invoiceNo,
		}
		if err := tx.Create(&receipt).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		// Process each line: load PO item, validate qty, create batch + stock_movement.
		for _, line := range req.Msg.Lines {
			if line.Qty <= 0 {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("qty must be > 0"))
			}
			expiry, err := time.Parse("2006-01-02", line.ExpiryDate)
			if err != nil {
				return connect.NewError(connect.CodeInvalidArgument,
					fmt.Errorf("expiry_date must be YYYY-MM-DD: %w", err))
			}

			var poItem model.PurchaseOrderItem
			err = tx.Where("id = ? AND purchase_order_id = ?",
				line.PurchaseOrderItemId, po.ID).First(&poItem).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeInvalidArgument,
					errors.New("purchase_order_item not found on this PO"))
			}
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			// Resolve the purchasable unit (default to the PO line's unit) and
			// convert the entered qty to BASE units for all stock math.
			unitID := line.ProductUnitId
			if unitID == "" && poItem.ProductUnitID != nil {
				unitID = *poItem.ProductUnitID
			}
			unit, err := resolvePurchaseUnit(tx, poItem.ProductID, unitID)
			if err != nil {
				return err
			}
			baseQty := line.Qty * int32(unit.Factor)

			remaining := poItem.OrderedQty - poItem.ReceivedQty
			if baseQty > remaining {
				return connect.NewError(connect.CodeFailedPrecondition,
					fmt.Errorf("line qty %d %s (%d base) exceeds remaining %d base for product %s",
						line.Qty, unit.Name, baseQty, remaining, poItem.ProductID))
			}

			unitCost := line.UnitCostPrice
			if unitCost == 0 {
				unitCost = poItem.UnitCostPrice
			}

			// Create the batch row carrying supplier + cost + expiry.
			supplierID := po.SupplierID
			batch := model.Batch{
				ProductID:   poItem.ProductID,
				SupplierID:  &supplierID,
				BatchNumber: strings.TrimSpace(line.BatchNumber),
				ExpiryDate:  expiry,
				CostPrice:   unitCost,
				ReceivedAt:  receivedAt,
			}
			if err := tx.Create(&batch).Error; err != nil {
				return connect.NewError(connect.CodeInternal,
					fmt.Errorf("create batch: %w", err))
			}

			// PURCHASE stock_movement linked to this batch, into the active warehouse.
			mv := model.StockMovement{
				BatchID:     batch.ID,
				Qty:         baseQty,
				Type:        "PURCHASE",
				Reason:      fmt.Sprintf("PO %s receipt %s", common.Deref(po.PoNo), receiptNo),
				UserID:      caller.UserID,
				WarehouseID: warehouseID,
			}
			if err := tx.Create(&mv).Error; err != nil {
				return connect.NewError(connect.CodeInternal,
					fmt.Errorf("create stock movement: %w", err))
			}

			// Receipt item row linking back to the batch we just created.
			batchID := batch.ID
			unitRef := unit.ID
			rcvItem := model.PurchaseReceiptItem{
				PurchaseReceiptID:   receipt.ID,
				PurchaseOrderItemID: poItem.ID,
				ProductID:           poItem.ProductID,
				Qty:                 baseQty,
				UnitCostPrice:       unitCost,
				BatchNumber:         strings.TrimSpace(line.BatchNumber),
				ExpiryDate:          expiry,
				BatchID:             &batchID,
				ProductUnitID:       &unitRef,
				UnitName:            unit.Name,
				UnitFactor:          unit.Factor,
			}
			if err := tx.Create(&rcvItem).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			// Bump received_qty on the PO item.
			if err := tx.Model(&model.PurchaseOrderItem{}).
				Where("id = ?", poItem.ID).
				Update("received_qty", gorm.Expr("received_qty + ?", baseQty)).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		// Recompute PO status now that received_qty has changed.
		if err := recomputePOStatus(tx, &po); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	full, err := p.loadFull(ctx, receipt.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.CreateReceiptResponse{Receipt: receiptToProto(full)}), nil
}

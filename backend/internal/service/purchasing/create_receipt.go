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
		return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.po_required")
	}
	if len(req.Msg.Lines) == 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.lines_required")
	}

	// Stock lands in the PO's warehouse (stamped at CreatePurchaseOrder time),
	// not the caller's active warehouse. Loaded inside the tx below.
	var warehouseID string

	// Default receipt date to today.
	receivedAt := time.Now()
	if req.Msg.ReceivedAt != "" {
		t, err := time.Parse("2006-01-02", req.Msg.ReceivedAt)
		if err != nil {
			return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.received_at_invalid")
		}
		receivedAt = t
	}

	var receipt model.PurchaseReceipt

	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the PO for the whole receipt, as every other handler that moves its
		// status or received_qty does. The status check below is only worth
		// anything if the status can't change under it: unlocked, a void that
		// committed mid-receipt went unseen, the stock landed anyway, and
		// recomputePOStatus flipped the VOIDED order back to PARTIALLY_RECEIVED
		// (Postgres READ COMMITTED; SQLite's single-writer pool serializes it).
		// Two receipts racing each other were already serialized by the receipt
		// counter's row lock, but only by accident of statement order.
		po, err := (&PurchaseOrders{db: tx}).lockByID(tx, req.Msg.PurchaseOrderId)
		if err != nil {
			return err
		}
		switch po.Status {
		case poStatusSent, poStatusPartiallyReceived:
			// receivable — proceed
		default:
			return common.TokenError(connect.CodeFailedPrecondition, "purchasing.po_not_receivable")
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

		// Pharmacy rules on expiry are read once per receipt, not per line.
		pharmacy, err := common.IsPharmacyMode(ctx, tx)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		// Process each line: load PO item, validate qty, create batch + stock_movement.
		for _, line := range req.Msg.Lines {
			if line.Qty <= 0 {
				return common.TokenError(connect.CodeInvalidArgument, "purchasing.qty_invalid")
			}
			// The frontend seeds the product's default and reports whether it was
			// typed over; NONE (goods that do not expire) stores the placeholder.
			expirySource := common.ExpirySourceFromWire(int32(line.ExpirySource))
			expiry, ok := common.LotExpiry(line.ExpiryDate, expirySource)
			if !ok {
				return common.TokenError(connect.CodeInvalidArgument, "purchasing.expiry_invalid")
			}

			var poItem model.PurchaseOrderItem
			err = tx.Where("id = ? AND purchase_order_id = ?",
				line.PurchaseOrderItemId, po.ID).First(&poItem).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.TokenError(connect.CodeInvalidArgument, "purchasing.po_item_not_found")
			}
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			// In pharmacy mode an apotek must not take expired medicine onto the
			// shelf, nor record a prescription medicine as never expiring (FEFO
			// would sell it last and it would never show as expiring). Retail
			// only warns, in the dialog. Enforced here too because a rule the
			// client alone enforces is a rule that decays.
			if pharmacy {
				if expirySource == common.ExpirySourceNone {
					allowed, e := common.NoExpiryAllowed(ctx, tx, poItem.ProductID)
					if e != nil {
						return connect.NewError(connect.CodeInternal, e)
					}
					if !allowed {
						return common.TokenError(connect.CodeFailedPrecondition, "purchasing.expiry_required")
					}
				} else if common.ExpiryPassed(expiry, time.Now()) {
					return common.TokenError(connect.CodeFailedPrecondition, "purchasing.expiry_past")
				}
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
				return common.TokenError(connect.CodeFailedPrecondition, "purchasing.receive_exceeds_remaining")
			}

			// Cost the batch carries, via the one shared derivation (see
			// common.NetUnitCost): NET of the PO line's discount and INCLUSIVE of
			// PPN, so COGS and margins reflect what the stock actually cost.
			//
			// unit_cost_price on the receipt line is a true override — the UI
			// leaves it 0 so this derivation runs. When an operator does type
			// one, it is read on the same basis as the PO's entered costs
			// (per base unit, PPN-exclusive) and PPN is applied to it too, so
			// the tax stays a uniform PO-level property instead of depending on
			// which lines happened to be overridden.
			ppnRate := int32(0)
			if po.PpnEnabled {
				ppnRate = po.PpnRate
			}
			var unitCost int64
			if line.UnitCostPrice != 0 {
				unitCost = common.NetUnitCost(0, 0, line.UnitCostPrice, ppnRate)
			} else {
				unitCost = common.NetUnitCost(
					poItem.Subtotal, int64(poItem.OrderedQty), poItem.UnitCostPrice, ppnRate)
			}

			// Create the batch row carrying supplier + maker + cost + expiry.
			//
			// The MAKER comes from the purchase-order line, not from the product:
			// a product may have several approved pabrik and only the line records
			// which one this delivery is. This is the single point where that
			// answer becomes a permanent fact about physical stock -- every recall,
			// every "whose paracetamol is this" and every Pabrik filter on the
			// batches page reads the column written here. A line that names no
			// pabrik yields a lot that names none either, rather than one guessed
			// from the catalog: an unverified name on a lot is worse than a blank,
			// because only the blank is recognisable as not-recorded.
			supplierID := po.SupplierID
			batch := model.Batch{
				ProductID:      poItem.ProductID,
				SupplierID:     &supplierID,
				ManufacturerID: poItem.ManufacturerID,
				BatchNumber:    strings.TrimSpace(line.BatchNumber),
				ExpiryDate:     expiry,
				ExpirySource:   expirySource,
				CostPrice:      unitCost,
				ReceivedAt:     receivedAt,
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

			// Record the restock: upsert the last-value row + append a log row
			// (drives supplier detail, product-list columns, restock-history tab).
			if err := recordRestock(tx, restockEntry{
				WarehouseID:     warehouseID,
				ProductID:       poItem.ProductID,
				SupplierID:      supplierID,
				Price:           unitCost,
				Qty:             int64(baseQty),
				DiscountType:    poItem.DiscountType,
				DiscountValue:   poItem.DiscountValue,
				DiscountPerItem: poItem.DiscountPerItem,
				// Same source as the lot's maker a few lines up: the order line,
				// not the catalog. A line naming none yields a log row naming none.
				ManufacturerID:  poItem.ManufacturerID,
				CreatedAt:       po.CreatedAt, // PO (restock order) created
				ArrivedAt:       receivedAt,   // receipt received_at
				ReceiptID:       receipt.ID,
			}); err != nil {
				return err
			}

			// Bump received_qty on the PO item.
			if err := tx.Model(&model.PurchaseOrderItem{}).
				Where("id = ?", poItem.ID).
				Update("received_qty", gorm.Expr("received_qty + ?", baseQty)).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		// Recompute PO status now that received_qty has changed.
		if err := recomputePOStatus(tx, po); err != nil {
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
	out := receiptToProto(full)
	if err := attachExpirySources(ctx, p.db, out); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&purchasingifacev1.CreateReceiptResponse{Receipt: out}), nil
}

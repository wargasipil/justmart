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
	receivedAt, err := checkReceiptInput(req.Msg.Lines, req.Msg.ReceivedAt)
	if err != nil {
		return nil, err
	}

	var receipt *model.PurchaseReceipt
	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var e error
		receipt, e = createReceiptTx(ctx, tx, caller, req.Msg, receivedAt)
		return e
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

// checkReceiptInput is the part of a receive request that needs no database:
// at least one line, and a received date that parses (empty = today). Run
// before the transaction opens, by CreateReceipt and ReplaceReceipt alike.
func checkReceiptInput(lines []*purchasingifacev1.ReceiveLineInput, receivedAtStr string) (time.Time, error) {
	if len(lines) == 0 {
		return time.Time{}, common.TokenError(connect.CodeInvalidArgument, "purchasing.lines_required")
	}
	// Default receipt date to today.
	receivedAt := time.Now()
	if receivedAtStr != "" {
		t, err := time.Parse("2006-01-02", receivedAtStr)
		if err != nil {
			return time.Time{}, common.TokenError(connect.CodeInvalidArgument, "purchasing.received_at_invalid")
		}
		receivedAt = t
	}
	return receivedAt, nil
}

// createReceiptTx is CreateReceipt's work inside a caller-provided transaction:
// the PO lock and receivable check, the RCV number, and per line the lot, its
// PURCHASE movement, the receipt line, the restock record and received_qty,
// then the PO status. ReplaceReceipt runs it right after cancelReceiptTx in the
// same transaction, so the outstanding qty it checks against is the one left
// after the replaced delivery was given back. receivedAt comes from
// checkReceiptInput.
func createReceiptTx(
	ctx context.Context,
	tx *gorm.DB,
	caller auth.Principal,
	msg *purchasingifacev1.CreateReceiptRequest,
	receivedAt time.Time,
) (*model.PurchaseReceipt, error) {
	// Stock lands in the PO's warehouse (stamped at CreatePurchaseOrder time),
	// not the caller's active warehouse. Loaded below.
	var warehouseID string
	var receipt model.PurchaseReceipt

	// Lock the PO for the whole receipt, as every other handler that moves its
	// status or received_qty does. The status check below is only worth
	// anything if the status can't change under it: unlocked, a void that
	// committed mid-receipt went unseen, the stock landed anyway, and
	// recomputePOStatus flipped the VOIDED order back to PARTIALLY_RECEIVED
	// (Postgres READ COMMITTED; SQLite's single-writer pool serializes it).
	// Two receipts racing each other were already serialized by the receipt
	// counter's row lock, but only by accident of statement order.
	po, err := (&PurchaseOrders{db: tx}).lockByID(tx, msg.PurchaseOrderId)
	if err != nil {
		return nil, err
	}
	switch po.Status {
	case poStatusSent, poStatusPartiallyReceived:
		// receivable — proceed
	default:
		return nil, common.TokenError(connect.CodeFailedPrecondition, "purchasing.po_not_receivable")
	}
	// Pin stock destination to the PO's warehouse.
	warehouseID = po.WarehouseID

	// Create the receipt header with an assigned receipt_no.
	receiptNo, err := assignReceiptNo(tx, time.Now())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// Faktur is captured once at PO creation; the receive flow no longer asks
	// for it. Inherit the PO's invoice_no when the request omits one (an
	// explicit value still wins — e.g. a partial delivery with its own faktur
	// sent via the API).
	invoiceNo := strings.TrimSpace(msg.InvoiceNo)
	if invoiceNo == "" {
		invoiceNo = po.InvoiceNo
	}
	receipt = model.PurchaseReceipt{
		ReceiptNo:       &receiptNo,
		PurchaseOrderID: po.ID,
		ReceivedAt:      receivedAt,
		ReceivedBy:      caller.UserID,
		Note:            strings.TrimSpace(msg.Note),
		InvoiceNo:       invoiceNo,
	}
	if err := tx.Create(&receipt).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Pharmacy rules on expiry are read once per receipt, not per line.
	pharmacy, err := common.IsPharmacyMode(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Process each line: load PO item, validate qty, create batch + stock_movement.
	for _, line := range msg.Lines {
		if line.Qty <= 0 {
			return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.qty_invalid")
		}
		// The frontend seeds the product's default and reports whether it was
		// typed over; NONE (goods that do not expire) stores the placeholder.
		expirySource := common.ExpirySourceFromWire(int32(line.ExpirySource))
		expiry, ok := common.LotExpiry(line.ExpiryDate, expirySource)
		if !ok {
			return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.expiry_invalid")
		}

		var poItem model.PurchaseOrderItem
		err = tx.Where("id = ? AND purchase_order_id = ?",
			line.PurchaseOrderItemId, po.ID).First(&poItem).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.po_item_not_found")
		}
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
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
					return nil, connect.NewError(connect.CodeInternal, e)
				}
				if !allowed {
					return nil, common.TokenError(connect.CodeFailedPrecondition, "purchasing.expiry_required")
				}
			} else if common.ExpiryPassed(expiry, time.Now()) {
				return nil, common.TokenError(connect.CodeFailedPrecondition, "purchasing.expiry_past")
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
			return nil, err
		}
		baseQty := line.Qty * int32(unit.Factor)

		remaining := poItem.OrderedQty - poItem.ReceivedQty
		if baseQty > remaining {
			return nil, common.TokenError(connect.CodeFailedPrecondition, "purchasing.receive_exceeds_remaining")
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
		// The same derivation once per PACK, so the lot can show what one
		// box cost exactly (Rp 65.000, not cost_price × 30 = 65.010). Only
		// when the PO line carries its exact pack price and no override was
		// typed -- an override is a per-base figure with no pack behind it.
		// A line whose base qty is not whole packs (cannot happen through
		// CreatePurchaseOrder) is left unstamped rather than guessed at.
		// 0/0 tells every reader to derive from cost_price instead.
		var packCost, packFactor int64
		if line.UnitCostPrice == 0 && poItem.PackCostPrice > 0 && poItem.UnitFactor > 0 &&
			int64(poItem.OrderedQty)%poItem.UnitFactor == 0 {
			if packQty := int64(poItem.OrderedQty) / poItem.UnitFactor; packQty > 0 {
				packCost = common.NetUnitCost(poItem.Subtotal, packQty, poItem.PackCostPrice, ppnRate)
				packFactor = poItem.UnitFactor
			}
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
			CostPackPrice:  packCost,
			CostPackFactor: packFactor,
			ReceivedAt:     receivedAt,
		}
		if err := tx.Create(&batch).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal,
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
			return nil, connect.NewError(connect.CodeInternal,
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
			return nil, connect.NewError(connect.CodeInternal, err)
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
			ManufacturerID: poItem.ManufacturerID,
			CreatedAt:      po.CreatedAt, // PO (restock order) created
			ArrivedAt:      receivedAt,   // receipt received_at
			ReceiptID:      receipt.ID,
		}); err != nil {
			return nil, err
		}

		// Bump received_qty on the PO item.
		if err := tx.Model(&model.PurchaseOrderItem{}).
			Where("id = ?", poItem.ID).
			Update("received_qty", gorm.Expr("received_qty + ?", baseQty)).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Recompute PO status now that received_qty has changed.
	if err := recomputePOStatus(tx, po); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &receipt, nil
}

package purchasing

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (p *PurchaseReceipts) ListReceipts(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.ListReceiptsRequest],
) (*connect.Response[purchasingifacev1.ListReceiptsResponse], error) {
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)

	// One filter closure feeds both the count and the page, so the two can't
	// drift. Preload("Items") is on the PAGE query only — counting with a
	// preload would load every child row just to throw them away.
	applyFilters := func(q *gorm.DB) *gorm.DB {
		q = q.Model(&model.PurchaseReceipt{})
		if req.Msg.PurchaseOrderId != "" {
			q = q.Where("purchase_order_id = ?", req.Msg.PurchaseOrderId)
		}
		return q
	}
	var total int64
	if err := applyFilters(p.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.PurchaseReceipt
	if err := applyFilters(p.db.WithContext(ctx)).
		Preload("Items").
		Order("created_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*purchasingifacev1.PurchaseReceipt, 0, len(rows))
	for i := range rows {
		out = append(out, receiptToProto(&rows[i]))
	}
	// Surface each line's returnable max = its batch's current on-hand in the PO
	// warehouse (the purchase-return dialog caps qty by this). Only when scoped
	// to a single PO (the detail page always is). Enriches THIS PAGE's rows only,
	// which is the same per-page-enrich shape every other List* handler uses.
	if req.Msg.PurchaseOrderId != "" {
		if err := p.enrichReturnable(ctx, req.Msg.PurchaseOrderId, out); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&purchasingifacev1.ListReceiptsResponse{
		Receipts: out,
		Total:    int32(total),
	}), nil
}

// enrichReturnable sets returnable_qty (BASE units) on each receipt item = the
// current on-hand of its batch in the PO's warehouse, and the receipt-level
// cancellable / cancel_blocked_reason pair. Batched — three queries total,
// regardless of page size.
//
// Cancellability is precomputed here so the UI can disable the action with a
// reason instead of discovering it on click; CancelReceipt re-checks the same
// conditions under a row lock, which is where the decision is actually enforced.
func (p *PurchaseReceipts) enrichReturnable(ctx context.Context, poID string, receipts []*purchasingifacev1.PurchaseReceipt) error {
	var po model.PurchaseOrder
	if err := p.db.WithContext(ctx).Select("warehouse_id, status").Where("id = ?", poID).First(&po).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // unknown PO → nothing to enrich
		}
		return err
	}
	var batchIDs []string
	for _, r := range receipts {
		for _, it := range r.Items {
			if it.BatchId != "" {
				batchIDs = append(batchIDs, it.BatchId)
			}
		}
	}
	if len(batchIDs) == 0 {
		// No lots left to reason about (e.g. every receipt on the page is already
		// cancelled). Still stamp the blocked reason so the UI has copy to show.
		markCancellable(receipts, po.Status, nil, nil)
		return nil
	}
	type row struct {
		BatchID string `gorm:"column:batch_id"`
		OnHand  int64  `gorm:"column:on_hand"`
		Moves   int64  `gorm:"column:moves"`
	}
	var rows []row
	if err := p.db.WithContext(ctx).Model(&model.StockMovement{}).
		Select("batch_id, COALESCE(SUM(qty), 0) AS on_hand, COUNT(*) AS moves").
		Where("batch_id IN ? AND warehouse_id = ?", batchIDs, po.WarehouseID).
		Group("batch_id").Scan(&rows).Error; err != nil {
		return err
	}
	onHand := make(map[string]int64, len(rows))
	moves := make(map[string]int64, len(rows))
	for _, r := range rows {
		onHand[r.BatchID] = r.OnHand
		moves[r.BatchID] = r.Moves
	}

	// Lots held by an open stocktake can't be deleted (NOT NULL FK on
	// stocktake_lines.batch_id) — see cancelBlockedStocktake.
	var counted []struct {
		BatchID string `gorm:"column:batch_id"`
	}
	if err := p.db.WithContext(ctx).Model(&model.StocktakeLine{}).
		Select("DISTINCT batch_id").
		Where("batch_id IN ?", batchIDs).Scan(&counted).Error; err != nil {
		return err
	}
	inStocktake := make(map[string]bool, len(counted))
	for _, c := range counted {
		inStocktake[c.BatchID] = true
	}

	for _, r := range receipts {
		for _, it := range r.Items {
			if it.BatchId != "" {
				it.ReturnableQty = onHand[it.BatchId]
			}
		}
	}
	markCancellable(receipts, po.Status, moves, inStocktake)
	return nil
}

// markCancellable stamps cancellable + cancel_blocked_reason on each receipt.
// A receipt is cancellable only when every one of its lots is untouched: exactly
// one stock movement (the PURCHASE that created it) for the full received qty.
// Mirrors CancelReceipt's guards, in the same precedence order.
func markCancellable(
	receipts []*purchasingifacev1.PurchaseReceipt,
	poStatus string,
	moves map[string]int64,
	inStocktake map[string]bool,
) {
	for _, r := range receipts {
		switch {
		case r.VoidedAt != 0:
			r.CancelBlockedReason = cancelBlockedVoided
		case poStatus == poStatusClosed:
			r.CancelBlockedReason = cancelBlockedPOClosed
		default:
			r.Cancellable = true
			for _, it := range r.Items {
				if it.BatchId == "" || moves[it.BatchId] != 1 || it.ReturnableQty != int64(it.Qty) {
					r.Cancellable = false
					r.CancelBlockedReason = cancelBlockedConsumed
					break
				}
				if inStocktake[it.BatchId] {
					r.Cancellable = false
					r.CancelBlockedReason = cancelBlockedStocktake
					break
				}
			}
		}
	}
}

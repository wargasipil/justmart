package purchasing

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (p *PurchaseReceipts) ListReceipts(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.ListReceiptsRequest],
) (*connect.Response[purchasingifacev1.ListReceiptsResponse], error) {
	q := p.db.WithContext(ctx).Preload("Items").Order("created_at DESC")
	if req.Msg.PurchaseOrderId != "" {
		q = q.Where("purchase_order_id = ?", req.Msg.PurchaseOrderId)
	}
	var rows []model.PurchaseReceipt
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*purchasingifacev1.PurchaseReceipt, 0, len(rows))
	for i := range rows {
		out = append(out, receiptToProto(&rows[i]))
	}
	// Surface each line's returnable max = its batch's current on-hand in the PO
	// warehouse (the purchase-return dialog caps qty by this). Only when scoped
	// to a single PO (the detail page always is).
	if req.Msg.PurchaseOrderId != "" {
		if err := p.enrichReturnable(ctx, req.Msg.PurchaseOrderId, out); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&purchasingifacev1.ListReceiptsResponse{Receipts: out}), nil
}

// enrichReturnable sets returnable_qty (BASE units) on each receipt item = the
// current on-hand of its batch in the PO's warehouse. One batched query.
func (p *PurchaseReceipts) enrichReturnable(ctx context.Context, poID string, receipts []*purchasingifacev1.PurchaseReceipt) error {
	var po model.PurchaseOrder
	if err := p.db.WithContext(ctx).Select("warehouse_id").Where("id = ?", poID).First(&po).Error; err != nil {
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
		return nil
	}
	type row struct {
		BatchID string `gorm:"column:batch_id"`
		OnHand  int64  `gorm:"column:on_hand"`
	}
	var rows []row
	if err := p.db.WithContext(ctx).Model(&model.StockMovement{}).
		Select("batch_id, COALESCE(SUM(qty), 0) AS on_hand").
		Where("batch_id IN ? AND warehouse_id = ?", batchIDs, po.WarehouseID).
		Group("batch_id").Scan(&rows).Error; err != nil {
		return err
	}
	onHand := make(map[string]int64, len(rows))
	for _, r := range rows {
		onHand[r.BatchID] = r.OnHand
	}
	for _, r := range receipts {
		for _, it := range r.Items {
			if it.BatchId != "" {
				it.ReturnableQty = onHand[it.BatchId]
			}
		}
	}
	return nil
}

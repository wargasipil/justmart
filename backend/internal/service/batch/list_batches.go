package batch

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *BatchService) ListBatches(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListBatchesRequest],
) (*connect.Response[inventoryifacev1.ListBatchesResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}

	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)

	// Per-warehouse stock is computed in SQL (GROUP BY) so only_in_stock +
	// pagination + total stay consistent.
	base := func() *gorm.DB {
		q := s.db.WithContext(ctx).
			Table("batches AS b").
			Joins("LEFT JOIN stock_movements sm ON sm.batch_id = b.id AND sm.warehouse_id = ?", warehouseID).
			Group("b.id")
		if req.Msg.ProductId != "" {
			q = q.Where("b.product_id = ?", req.Msg.ProductId)
		}
		if sid := strings.TrimSpace(req.Msg.SupplierId); sid != "" {
			q = q.Where("b.supplier_id = ?", sid)
		}
		if req.Msg.OnlyInStock {
			q = q.Having("COALESCE(SUM(sm.qty), 0) > 0")
		}
		if query := strings.TrimSpace(req.Msg.Query); query != "" {
			pattern := "%" + query + "%"
			sub := s.db.Table("batches AS b2").
				Select("b2.id").
				Joins("JOIN products m ON m.id = b2.product_id").
				Where("b2.batch_number "+common.LikeOp(s.db)+" ? OR m.name "+common.LikeOp(s.db)+" ? OR m.sku "+common.LikeOp(s.db)+" ?", pattern, pattern, pattern)
			q = q.Where("b.id IN (?)", sub)
		}
		if req.Msg.FromUnix > 0 || req.Msg.ToUnix > 0 {
			col := "b.received_at"
			if req.Msg.DateField == "expiry" {
				col = "b.expiry_date"
			}
			if req.Msg.FromUnix > 0 {
				q = q.Where(col+" >= ?", time.Unix(req.Msg.FromUnix, 0))
			}
			if req.Msg.ToUnix > 0 {
				q = q.Where(col+" < ?", time.Unix(req.Msg.ToUnix, 0))
			}
		}
		return q
	}

	var total int64
	if err := s.db.WithContext(ctx).
		Table("(?) AS sub", base().Select("b.id")).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	type batchRow struct {
		model.Batch
		Qty int64 `gorm:"column:qty"`
	}
	var rows []batchRow
	if err := base().
		Select("b.*, COALESCE(SUM(sm.qty), 0) AS qty").
		Order("b.expiry_date ASC").Offset(offset).Limit(limit).Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Batch, 0, len(rows))
	for i := range rows {
		out = append(out, batchToProto(&rows[i].Batch, rows[i].Qty))
	}
	// Enrich each row with its originating PO (via the receipt that created
	// it). Empty for legacy / manual CreateBatch path. One batched query.
	if len(out) > 0 {
		batchIDs := make([]string, 0, len(out))
		for _, x := range out {
			batchIDs = append(batchIDs, x.Id)
		}
		type poRow struct {
			BatchID         string `gorm:"column:batch_id"`
			PurchaseOrderID string `gorm:"column:purchase_order_id"`
			PoNo            string `gorm:"column:po_no"`
		}
		var poRows []poRow
		if err := s.db.WithContext(ctx).
			Table("purchase_receipt_items AS pri").
			Select("pri.batch_id AS batch_id, po.id AS purchase_order_id, COALESCE(po.po_no, '') AS po_no").
			Joins("JOIN purchase_receipts pr ON pr.id = pri.purchase_receipt_id").
			Joins("JOIN purchase_orders po ON po.id = pr.purchase_order_id").
			Where("pri.batch_id IN ?", batchIDs).
			Scan(&poRows).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		byBatch := make(map[string]poRow, len(poRows))
		for _, r := range poRows {
			byBatch[r.BatchID] = r
		}
		for _, x := range out {
			if r, ok := byBatch[x.Id]; ok {
				x.PurchaseOrderId = r.PurchaseOrderID
				x.PoNo = r.PoNo
			}
		}
	}
	return connect.NewResponse(&inventoryifacev1.ListBatchesResponse{
		Batches: out,
		Total:   int32(total),
	}), nil
}

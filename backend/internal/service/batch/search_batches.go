package batch

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *BatchService) SearchBatches(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.SearchBatchesRequest],
) (*connect.Response[inventoryifacev1.SearchBatchesResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	// Scope the per-batch stock to the requested warehouse when provided (e.g.
	// the transfer picker scopes to the chosen source warehouse), else fall back
	// to the caller's active warehouse. Mirrors resolveWarehouse's trust model.
	warehouseID := strings.TrimSpace(req.Msg.WarehouseId)
	if warehouseID == "" {
		warehouseID, err = common.ResolveWarehouse(ctx, s.db, caller)
		if err != nil {
			return nil, err
		}
	}
	query := strings.TrimSpace(req.Msg.Query)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	// Per-warehouse stock is computed in SQL (GROUP BY) so only_in_stock + limit
	// stay consistent (filtering after the limit would silently truncate).
	q := s.db.WithContext(ctx).
		Table("batches AS b").
		Joins("JOIN products AS m ON m.id = b.product_id").
		Joins("LEFT JOIN stock_movements sm ON sm.batch_id = b.id AND sm.warehouse_id = ?", warehouseID).
		Group("b.id, m.name").
		Order("b.expiry_date ASC").
		Limit(limit).
		Select("b.*, m.name AS product_name, COALESCE(SUM(sm.qty), 0) AS qty")
	if req.Msg.ProductId != "" {
		q = q.Where("b.product_id = ?", req.Msg.ProductId)
	}
	if query != "" {
		pattern := "%" + query + "%"
		q = q.Where("b.batch_number "+common.LikeOp(s.db)+" ? OR m.name "+common.LikeOp(s.db)+" ? OR m.sku "+common.LikeOp(s.db)+" ?", pattern, pattern, pattern)
	}
	if req.Msg.OnlyInStock {
		q = q.Having("COALESCE(SUM(sm.qty), 0) > 0")
	}

	type batchRow struct {
		model.Batch
		ProductName string `gorm:"column:product_name"`
		Qty         int64  `gorm:"column:qty"`
	}
	var rows []batchRow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Batch, 0, len(rows))
	for i := range rows {
		pb := batchToProto(&rows[i].Batch, rows[i].Qty)
		pb.ProductName = rows[i].ProductName
		out = append(out, pb)
	}
	// Enrich with each product's active units so pickers can offer per-line unit
	// entry (e.g. transfer "2 box"). Batch-loaded, no N+1.
	if err := s.attachUnits(ctx, out); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.SearchBatchesResponse{Batches: out}), nil
}

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
	order, err := listBatchesOrder(caller, req.Msg.SortBy, req.Msg.SortDesc)
	if err != nil {
		return nil, err
	}

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
		// Who MADE the stock, which is a different question from who sold it:
		// a recall names the pabrik and a lot is the only row that records one.
		if mid := strings.TrimSpace(req.Msg.ManufacturerId); mid != "" {
			q = q.Where("b.manufacturer_id = ?", mid)
		}
		if sid := strings.TrimSpace(req.Msg.SupplierId); sid != "" {
			q = q.Where("b.supplier_id = ?", sid)
		}
		// Where the expiry came from. DEFAULT is the shelf-check worklist: dates
		// that are the product's estimate because nobody typed over them.
		if req.Msg.ExpirySource != inventoryifacev1.ExpirySource_EXPIRY_SOURCE_UNSPECIFIED {
			q = q.Where("b.expiry_source = ?", common.ExpirySourceFromWire(int32(req.Msg.ExpirySource)))
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
		Order(order).Offset(offset).Limit(limit).Scan(&rows).Error; err != nil {
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
	redactCost(caller, out)
	return connect.NewResponse(&inventoryifacev1.ListBatchesResponse{
		Batches: out,
		Total:   int32(total),
	}), nil
}

// listBatchesSortColumns whitelists ListBatchesRequest.sort_by -> its ORDER BY
// expression. Only these constant strings ever reach Order(); the request
// value is a map KEY, never interpolated into SQL.
//
// Each expression is legal under the query's GROUP BY b.id on SQLite AND
// Postgres: b.* columns are functionally dependent on the primary key, the
// supplier name is a correlated subquery (so no join has to be grouped), and
// qty orders by the aggregate EXPRESSION rather than its "qty" alias. The
// supplier subquery is COALESCEd to the empty string so a lot with no supplier
// sorts in the same place on both engines (SQLite puts NULL first ascending,
// Postgres last).
var listBatchesSortColumns = map[string]string{
	"batch_number": "b.batch_number",
	"supplier":     "COALESCE((SELECT s.name FROM suppliers s WHERE s.id = b.supplier_id), '')",
	"expiry":       "b.expiry_date",
	"cost":         "b.cost_price",
	"qty":          "COALESCE(SUM(sm.qty), 0)",
}

// listBatchesDefaultOrder is soonest expiry first. Like every sorted order here
// it ends in b.id so equal keys keep one stable order and rows never shuffle
// between pages (OFFSET paging over a non-unique key is otherwise undefined).
const listBatchesDefaultOrder = "b.expiry_date ASC, b.id ASC"

// listBatchesOrder resolves the request's sort into an ORDER BY clause.
// "" is the default order; an unknown column is refused with a stable token.
func listBatchesOrder(caller auth.Principal, sortBy string, desc bool) (string, error) {
	sortBy = strings.TrimSpace(sortBy)
	if sortBy == "" {
		return listBatchesDefaultOrder, nil
	}
	expr, ok := listBatchesSortColumns[sortBy]
	if !ok {
		return "", common.TokenError(connect.CodeInvalidArgument, "batch.sort_invalid")
	}
	// Cost and supplier are the fields redactCost blanks for a caller who may
	// not see cost. Ordering rows by a value the response then hides would
	// still leak it (the row order IS the ranking), so such a caller gets the
	// default order instead. Every role may see cost today (common.CanSeeCost);
	// this keeps the sort honest if that predicate is ever narrowed again.
	if (sortBy == "cost" || sortBy == "supplier") && !common.CanSeeCost(caller.Role) {
		return listBatchesDefaultOrder, nil
	}
	dir := " ASC"
	if desc {
		dir = " DESC"
	}
	return expr + dir + ", b.id ASC", nil
}

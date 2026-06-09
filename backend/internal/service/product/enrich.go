package product

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// enrichStock fills ready_stock (on-hand in the active warehouse) + on_order_stock
// (incoming on open POs) for a page of products. Two batched grouped queries.
func (s *ProductService) enrichStock(
	ctx context.Context,
	caller auth.Principal,
	meds []*inventoryifacev1.Product,
) error {
	if len(meds) == 0 {
		return nil
	}
	ids := make([]string, 0, len(meds))
	for _, md := range meds {
		ids = append(ids, md.Id)
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return err
	}

	// Ready: SUM(stock_movements.qty) per product in the active warehouse.
	type readyRow struct {
		ProductID string `gorm:"column:product_id"`
		Qty       int64  `gorm:"column:qty"`
	}
	var readyRows []readyRow
	if err := s.db.WithContext(ctx).
		Table("batches AS b").
		Select("b.product_id AS product_id, COALESCE(SUM(sm.qty), 0) AS qty").
		Joins("LEFT JOIN stock_movements sm ON sm.batch_id = b.id AND sm.warehouse_id = ?", warehouseID).
		Where("b.product_id IN ?", ids).
		Group("b.product_id").Scan(&readyRows).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	ready := make(map[string]int64, len(readyRows))
	for _, r := range readyRows {
		ready[r.ProductID] = r.Qty
	}

	// On-order: SUM(ordered_qty - received_qty) per product on open POs.
	type orderRow struct {
		ProductID string `gorm:"column:product_id"`
		Qty       int64  `gorm:"column:qty"`
	}
	var orderRows []orderRow
	if err := s.db.WithContext(ctx).
		Table("purchase_order_items AS poi").
		Select("poi.product_id AS product_id, COALESCE(SUM(poi.ordered_qty - poi.received_qty), 0) AS qty").
		Joins("JOIN purchase_orders po ON po.id = poi.purchase_order_id").
		Where("poi.product_id IN ? AND po.status NOT IN ?", ids,
			[]string{common.POStatusVoided, common.POStatusClosed, common.POStatusReceived}).
		Group("poi.product_id").Scan(&orderRows).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	onOrder := make(map[string]int64, len(orderRows))
	for _, r := range orderRows {
		onOrder[r.ProductID] = r.Qty
	}

	for _, md := range meds {
		md.ReadyStock = ready[md.Id]
		md.OnOrderStock = onOrder[md.Id]
	}
	return nil
}

// enrichLastStocktake fills last_stocktake_date for a page of products: the
// most recent COMPLETED stocktake that touched any batch of the product in
// the caller's active warehouse.
func (s *ProductService) enrichLastStocktake(
	ctx context.Context,
	caller auth.Principal,
	meds []*inventoryifacev1.Product,
) error {
	if len(meds) == 0 {
		return nil
	}
	ids := make([]string, 0, len(meds))
	for _, md := range meds {
		ids = append(ids, md.Id)
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return err
	}
	type opnameRow struct {
		ProductID   string `gorm:"column:product_id"`
		CompletedAt string `gorm:"column:completed_at"`
	}
	var rows []opnameRow
	if err := s.db.WithContext(ctx).
		Table("stocktake_sessions AS ss").
		Select("b.product_id AS product_id, "+common.DayKeyExpr(s.db, "MAX(ss.completed_at)")+" AS completed_at").
		Joins("JOIN stocktake_lines sl ON sl.session_id = ss.id").
		Joins("JOIN batches b ON b.id = sl.batch_id").
		Where("ss.warehouse_id = ? AND ss.status = ? AND sl.counted_qty IS NOT NULL AND b.product_id IN ?",
			warehouseID, "COMPLETED", ids).
		Group("b.product_id").
		Scan(&rows).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	byID := make(map[string]string, len(rows))
	for _, r := range rows {
		byID[r.ProductID] = r.CompletedAt
	}
	for _, md := range meds {
		if d, ok := byID[md.Id]; ok && d != "" {
			md.LastStocktakeDate = d
		}
	}
	return nil
}

// attachUnits batch-loads each product's active units (base first, then by
// factor) and sets them on the protos. No N+1.
func (s *ProductService) attachUnits(ctx context.Context, meds []*inventoryifacev1.Product) error {
	if len(meds) == 0 {
		return nil
	}
	ids := make([]string, 0, len(meds))
	for _, md := range meds {
		ids = append(ids, md.Id)
	}
	var rows []model.ProductUnit
	if err := s.db.WithContext(ctx).
		Where("product_id IN ? AND active", ids).
		Order("is_base DESC, factor ASC").
		Find(&rows).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	byMed := make(map[string][]*inventoryifacev1.ProductUnit, len(meds))
	for i := range rows {
		byMed[rows[i].ProductID] = append(byMed[rows[i].ProductID], productUnitToProto(&rows[i]))
	}
	for _, md := range meds {
		md.Units = byMed[md.Id]
	}
	return nil
}

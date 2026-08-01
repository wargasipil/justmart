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

	// On-order: outstanding qty + its value at cost, per product on open POs.
	// Fetched per LINE rather than pre-summed in SQL because the valuation rate
	// is the line's NET per-base cost, whose rounding must match the one
	// CreateReceipt stamps on the batch — see netUnitCost.
	type orderRow struct {
		ProductID   string `gorm:"column:product_id"`
		OrderedQty  int64  `gorm:"column:ordered_qty"`
		ReceivedQty int64  `gorm:"column:received_qty"`
		Subtotal    int64  `gorm:"column:subtotal"`
		UnitCost    int64  `gorm:"column:unit_cost_price"`
	}
	var orderRows []orderRow
	if err := s.db.WithContext(ctx).
		Table("purchase_order_items AS poi").
		Select("poi.product_id AS product_id, poi.ordered_qty AS ordered_qty, "+
			"poi.received_qty AS received_qty, poi.subtotal AS subtotal, "+
			"poi.unit_cost_price AS unit_cost_price").
		Joins("JOIN purchase_orders po ON po.id = poi.purchase_order_id").
		Where("poi.product_id IN ? AND po.status NOT IN ?", ids,
			[]string{common.POStatusVoided, common.POStatusClosed, common.POStatusReceived}).
		Scan(&orderRows).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	onOrder := make(map[string]int64, len(orderRows))
	onOrderValue := make(map[string]int64, len(orderRows))
	for _, r := range orderRows {
		outstanding := r.OrderedQty - r.ReceivedQty
		if outstanding <= 0 {
			continue
		}
		onOrder[r.ProductID] += outstanding
		onOrderValue[r.ProductID] += outstanding * netUnitCost(r.Subtotal, r.OrderedQty, r.UnitCost)
	}

	for _, md := range meds {
		md.ReadyStock = ready[md.Id]
		md.OnOrderStock = onOrder[md.Id]
		md.OnOrderValuation = onOrderValue[md.Id]
	}
	return nil
}

// netUnitCost is the per-base-unit cost of a PO line after its line discount.
// Subtotal is NET and orderedQty is in BASE units, so this is what a receipt
// will write to batches.cost_price — keeping "ongoing" valuation on the same
// basis as the "ready" valuation it becomes. Mirrors CreateReceipt's derivation
// exactly (including the half-up rounding); gross is the fallback when the line
// carries no qty to divide by.
func netUnitCost(subtotal, orderedQty, gross int64) int64 {
	if orderedQty <= 0 {
		return gross
	}
	return (subtotal + orderedQty/2) / orderedQty
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

// enrichLastRestock fills the last_restock_* fields for a page of products: the
// single most-recent restock across all suppliers for the product in the active
// warehouse, from product_last_restocks. Empty until a receipt records one.
func (s *ProductService) enrichLastRestock(
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
	var rows []model.ProductLastRestock
	if err := s.db.WithContext(ctx).
		Where("warehouse_id = ? AND product_id IN ?", warehouseID, ids).
		Order("last_arrived_at DESC, updated_at DESC").
		Find(&rows).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	// Rows are newest-first; keep the first (= latest) seen per product.
	byID := make(map[string]*model.ProductLastRestock, len(rows))
	for i := range rows {
		r := &rows[i]
		if _, ok := byID[r.ProductID]; !ok {
			byID[r.ProductID] = r
		}
	}
	for _, md := range meds {
		r, ok := byID[md.Id]
		if !ok {
			continue
		}
		md.LastRestockPrice = r.LastPrice
		md.LastRestockQty = r.LastQty
		md.LastRestockDiscountType = r.LastDiscountType
		md.LastRestockDiscountValue = r.LastDiscountValue
		md.LastRestockCreatedAt = r.LastCreatedAt.Unix()
		md.LastRestockArrivedAt = r.LastArrivedAt.Unix()
		md.LastRestockSupplierId = r.SupplierID
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
	// Grosir tiers are per-unit prices, so they hydrate with the units rather than
	// at their own call sites. Coupling them here is deliberate: POS reads the
	// catalog through ListProducts, and a tier set that hydrated only on
	// GetProduct would leave every POS wholesale hint silently blank.
	return s.attachPriceTiers(ctx, meds)
}

// attachPriceTiers batch-loads each product's grosir (wholesale) quantity price
// tiers and sets them on the protos, ordered base unit first, then by unit factor,
// then ascending threshold — the order the Grosir tab and POS render them in.
// Tiers of an archived unit are omitted (the join filters on pu.active), matching
// attachUnits so a hidden unit can't leave orphan tiers on screen. No N+1.
func (s *ProductService) attachPriceTiers(ctx context.Context, meds []*inventoryifacev1.Product) error {
	if len(meds) == 0 {
		return nil
	}
	ids := make([]string, 0, len(meds))
	for _, md := range meds {
		ids = append(ids, md.Id)
	}
	var rows []model.ProductPriceTier
	if err := s.db.WithContext(ctx).
		Model(&model.ProductPriceTier{}).
		Joins("JOIN product_units pu ON pu.id = product_price_tiers.product_unit_id AND pu.active").
		Where("product_price_tiers.product_id IN ?", ids).
		Order("pu.is_base DESC, pu.factor ASC, product_price_tiers.min_qty ASC").
		Find(&rows).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	byMed := make(map[string][]*inventoryifacev1.ProductPriceTier, len(meds))
	for i := range rows {
		byMed[rows[i].ProductID] = append(byMed[rows[i].ProductID], productPriceTierToProto(&rows[i]))
	}
	for _, md := range meds {
		md.PriceTiers = byMed[md.Id]
	}
	return nil
}

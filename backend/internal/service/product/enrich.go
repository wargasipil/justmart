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
	// A COMPOSITE product holds no batches of its own, so the join above leaves it
	// at 0. Its real "ready" is how many portions its ingredients can build.
	return s.overlayCompositeStock(ctx, caller, meds)
}

// overlayCompositeStock replaces ready_stock for COMPOSITE products with the
// number of BASE units their recipe components can currently build in the active
// warehouse (min over lines of floor(component_ready / qty_base)).
//
// Without this a menu item reads 0 everywhere — the product list, the low-stock
// bell, POS — because it genuinely has no batches, which is true and useless. Two
// batched queries regardless of page size; a page with no composites returns
// after the first cheap scan.
//
// A composite with an EMPTY recipe is reported as 0 rather than "unbounded":
// nothing is defined for it to consume, so it is misconfigured, and 0 is the
// reading that sends someone to the recipe card. SERVICE products are left at 0
// — they consume nothing by definition, and the UI omits the stock chip for them
// rather than showing a quantity that has no meaning.
func (s *ProductService) overlayCompositeStock(
	ctx context.Context,
	caller auth.Principal,
	meds []*inventoryifacev1.Product,
) error {
	compositeIDs := make([]string, 0, len(meds))
	for _, md := range meds {
		if md.Kind == inventoryifacev1.ProductKind_PRODUCT_KIND_COMPOSITE {
			compositeIDs = append(compositeIDs, md.Id)
		}
	}
	if len(compositeIDs) == 0 {
		return nil
	}

	var lines []model.ProductRecipeItem
	if err := s.db.WithContext(ctx).
		Where("product_id IN ?", compositeIDs).Find(&lines).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	byParent := make(map[string][]model.ProductRecipeItem, len(compositeIDs))
	componentIDs := make([]string, 0, len(lines))
	for i := range lines {
		byParent[lines[i].ProductID] = append(byParent[lines[i].ProductID], lines[i])
		componentIDs = append(componentIDs, lines[i].ComponentProductID)
	}

	componentReady, err := common.ReadyStockByProduct(ctx, s.db, caller, componentIDs)
	if err != nil {
		return err
	}
	for _, md := range meds {
		if md.Kind != inventoryifacev1.ProductKind_PRODUCT_KIND_COMPOSITE {
			continue
		}
		buildable := common.BuildablePortions(byParent[md.Id], componentReady)
		if buildable < 0 {
			buildable = 0 // no recipe defined — see above
		}
		md.ReadyStock = buildable
	}
	return nil
}

// attachRecipe batch-loads each COMPOSITE product's recipe lines and sets them on
// the protos, oldest first (the order a recipe is read in). Hydrated with the
// units/tiers rather than at its own call sites for the same reason those are:
// POS reads the catalog through ListProducts, and a Get-only hydration would
// leave every menu item's ingredient hint blank on the one screen that needs it.
//
// Display fields (component name/sku/unit) are filled here too — the POS "what's
// missing" hint names the ingredient. Component stock is NOT: the whole-recipe
// answer already arrives as ready_stock via overlayCompositeStock, and repeating
// the per-component figure on every catalog row would cost a second grouped
// query on the hot list path for something only the recipe card renders.
func (s *ProductService) attachRecipe(ctx context.Context, meds []*inventoryifacev1.Product) error {
	ids := make([]string, 0, len(meds))
	for _, md := range meds {
		if md.Kind == inventoryifacev1.ProductKind_PRODUCT_KIND_COMPOSITE {
			ids = append(ids, md.Id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var rows []model.ProductRecipeItem
	if err := s.db.WithContext(ctx).
		Where("product_id IN ?", ids).
		Order("created_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if len(rows) == 0 {
		return nil
	}

	componentIDs := make([]string, 0, len(rows))
	for i := range rows {
		componentIDs = append(componentIDs, rows[i].ComponentProductID)
	}
	var comps []model.Product
	if err := s.db.WithContext(ctx).
		Select("id", "sku", "name", "unit").
		Where("id IN ?", componentIDs).Find(&comps).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	byComponent := make(map[string]*model.Product, len(comps))
	for i := range comps {
		byComponent[comps[i].ID] = &comps[i]
	}

	byParent := make(map[string][]*inventoryifacev1.ProductRecipeItem, len(ids))
	for i := range rows {
		r := &rows[i]
		item := &inventoryifacev1.ProductRecipeItem{
			Id:                 r.ID,
			ProductId:          r.ProductID,
			ComponentProductId: r.ComponentProductID,
			QtyBase:            r.QtyBase,
			Note:               r.Note,
			CreatedAt:          r.CreatedAt.Unix(),
		}
		if c, ok := byComponent[r.ComponentProductID]; ok {
			item.ComponentName = c.Name
			item.ComponentSku = c.SKU
			item.ComponentUnit = c.Unit
		}
		byParent[r.ProductID] = append(byParent[r.ProductID], item)
	}
	for _, md := range meds {
		md.Recipe = byParent[md.Id]
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
	if err := s.attachPriceTiers(ctx, meds); err != nil {
		return err
	}
	// Same argument for recipes — a composite's ingredients are part of what the
	// catalog says about it, and POS needs them to explain an unavailable dish.
	return s.attachRecipe(ctx, meds)
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

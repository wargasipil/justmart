package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
)

type Products struct {
	db *gorm.DB
}

func NewProducts(db *gorm.DB) *Products { return &Products{db: db} }

func (m *Products) ListProducts(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductsRequest],
) (*connect.Response[inventoryifacev1.ListProductsResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset := normPage(req.Msg.Limit, req.Msg.Offset)
	query := strings.TrimSpace(req.Msg.Query)
	opnameBefore := strings.TrimSpace(req.Msg.OpnameBefore)
	if opnameBefore != "" {
		if _, perr := time.Parse(dateLayout, opnameBefore); perr != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("opname_before must be YYYY-MM-DD: %w", perr))
		}
	}
	// Resolve once — both the opname filter (active-warehouse-scoped) and the
	// downstream enrichStock use the same warehouse.
	warehouseID, err := resolveWarehouse(ctx, m.db, caller)
	if err != nil {
		return nil, err
	}

	applyFilters := func(q *gorm.DB) *gorm.DB {
		if req.Msg.OnlyArchived {
			q = q.Where("active = ?", false)
		} else if !req.Msg.IncludeInactive {
			q = q.Where("active = ?", true)
		}
		if query != "" {
			pattern := "%" + query + "%"
			q = q.Where("name "+likeOp(q)+" ? OR sku "+likeOp(q)+" ?", pattern, pattern)
		}
		if opnameBefore != "" {
			// "Last opname < before OR never counted" = "no completed opname
			// session with completed_at >= before touched this product in the
			// active warehouse". Inverted EXISTS keeps both branches.
			q = q.Where(`NOT EXISTS (
				SELECT 1 FROM stocktake_sessions ss
				JOIN stocktake_lines sl ON sl.session_id = ss.id
				JOIN batches b ON b.id = sl.batch_id
				WHERE ss.warehouse_id = ?
				  AND ss.status = 'COMPLETED'
				  AND ss.completed_at >= ?
				  AND sl.counted_qty IS NOT NULL
				  AND b.product_id = products.id
			)`, warehouseID, opnameBefore)
		}
		return q
	}

	var total int64
	if err := applyFilters(m.db.WithContext(ctx).Model(&model.Product{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.Product
	if err := applyFilters(m.db.WithContext(ctx).Model(&model.Product{})).
		Order("name").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Product, 0, len(rows))
	for i := range rows {
		out = append(out, productToProto(&rows[i]))
	}
	if err := m.enrichStock(ctx, caller, out); err != nil {
		return nil, err
	}
	if err := m.enrichLastStocktake(ctx, caller, out); err != nil {
		return nil, err
	}
	if err := m.attachUnits(ctx, out); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.ListProductsResponse{
		Products: out,
		Total:    int32(total),
	}), nil
}

// enrichStock fills ready_stock (on-hand in the active warehouse) + on_order_stock
// (incoming on open POs) for a page of products. Two batched grouped queries.
func (m *Products) enrichStock(
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
	warehouseID, err := resolveWarehouse(ctx, m.db, caller)
	if err != nil {
		return err
	}

	// Ready: SUM(stock_movements.qty) per product in the active warehouse.
	type readyRow struct {
		ProductID string `gorm:"column:product_id"`
		Qty       int64  `gorm:"column:qty"`
	}
	var readyRows []readyRow
	if err := m.db.WithContext(ctx).
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
	if err := m.db.WithContext(ctx).
		Table("purchase_order_items AS poi").
		Select("poi.product_id AS product_id, COALESCE(SUM(poi.ordered_qty - poi.received_qty), 0) AS qty").
		Joins("JOIN purchase_orders po ON po.id = poi.purchase_order_id").
		Where("poi.product_id IN ? AND po.status NOT IN ?", ids,
			[]string{poStatusVoided, poStatusClosed, poStatusReceived}).
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
// the caller's active warehouse. counted_qty IS NOT NULL keeps untouched lines
// from electing their session. Variance is intentionally NOT enriched on the
// list (kept on detail to keep the list column compact).
func (m *Products) enrichLastStocktake(
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
	warehouseID, err := resolveWarehouse(ctx, m.db, caller)
	if err != nil {
		return err
	}
	// Format the date in SQL (dayKeyExpr) and scan a string: SQLite returns a
	// bare datetime aggregate as a string GORM can't scan into time.Time, and
	// the proto wants a 'YYYY-MM-DD' string anyway.
	type opnameRow struct {
		ProductID   string `gorm:"column:product_id"`
		CompletedAt string `gorm:"column:completed_at"`
	}
	var rows []opnameRow
	if err := m.db.WithContext(ctx).
		Table("stocktake_sessions AS ss").
		Select("b.product_id AS product_id, "+dayKeyExpr(m.db, "MAX(ss.completed_at)")+" AS completed_at").
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

func (m *Products) GetProduct(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetProductRequest],
) (*connect.Response[inventoryifacev1.GetProductResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	med, err := m.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	warehouseID, err := resolveWarehouse(ctx, m.db, caller)
	if err != nil {
		return nil, err
	}
	out := productToProto(med)
	// Fill ready_stock (active warehouse) + on_order_stock so the detail page
	// shows the same figures as the list.
	if err := m.enrichStock(ctx, caller, []*inventoryifacev1.Product{out}); err != nil {
		return nil, err
	}
	// Last restock = the most recent stock arrival INTO THE ACTIVE WAREHOUSE for
	// this product (any positive movement: purchase, transfer-in, +adjustment),
	// with that batch's supplier. Detail-only (kept out of the list's enrichStock).
	var rr struct {
		ReceivedAt   time.Time `gorm:"column:received_at"`
		SupplierName string    `gorm:"column:supplier_name"`
	}
	if err := m.db.WithContext(ctx).
		Table("stock_movements sm").
		Select("b.received_at AS received_at, COALESCE(s.name, '') AS supplier_name").
		Joins("JOIN batches b ON b.id = sm.batch_id").
		Joins("LEFT JOIN suppliers s ON s.id = b.supplier_id").
		Where("b.product_id = ? AND sm.warehouse_id = ? AND sm.qty > 0", med.ID, warehouseID).
		Order("sm.created_at DESC").
		Limit(1).Scan(&rr).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !rr.ReceivedAt.IsZero() {
		out.LastRestockDate = rr.ReceivedAt.Format(dateLayout)
		out.LastRestockSupplier = rr.SupplierName
	}
	// Last opname = the most recent COMPLETED stocktake session that touched any
	// batch of this product in the active warehouse. Variance is sum(counted -
	// expected) over this product's counted lines in that session (base units;
	// negative = short, positive = surplus). counted_qty IS NOT NULL guard skips
	// untouched lines so they don't elect the session as "last touched".
	var st struct {
		CompletedAt time.Time `gorm:"column:completed_at"`
		Variance    int64     `gorm:"column:variance"`
	}
	if err := m.db.WithContext(ctx).
		Table("stocktake_sessions ss").
		Select("ss.completed_at AS completed_at, COALESCE(SUM(sl.counted_qty - sl.expected_qty), 0) AS variance").
		Joins("JOIN stocktake_lines sl ON sl.session_id = ss.id").
		Joins("JOIN batches b ON b.id = sl.batch_id").
		Where("ss.warehouse_id = ? AND ss.status = ? AND b.product_id = ? AND sl.counted_qty IS NOT NULL",
			warehouseID, "COMPLETED", med.ID).
		Group("ss.id, ss.completed_at").
		Order("ss.completed_at DESC").
		Limit(1).Scan(&st).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !st.CompletedAt.IsZero() {
		out.LastStocktakeDate = st.CompletedAt.Format(dateLayout)
		out.LastStocktakeVariance = st.Variance
	}
	// Total stock (on-hand in the active warehouse) + valuation at cost.
	// Σ(qty × cost) over movements == Σ_batch(qty)×cost (cost is per-batch).
	// Scoped to the active warehouse, so this equals ready_stock — the detail
	// page renders only valuation from it (the redundant "Total stock" tile is
	// gone); total_stock stays on the proto, just unrendered.
	var v struct {
		TotalStock int64 `gorm:"column:total_stock"`
		Valuation  int64 `gorm:"column:valuation"`
	}
	if err := m.db.WithContext(ctx).
		Table("stock_movements sm").
		Joins("JOIN batches b ON b.id = sm.batch_id").
		Where("b.product_id = ? AND sm.warehouse_id = ?", med.ID, warehouseID).
		Select("COALESCE(SUM(sm.qty), 0) AS total_stock, COALESCE(SUM(sm.qty * b.cost_price), 0) AS valuation").
		Scan(&v).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out.TotalStock = v.TotalStock
	out.StockValuation = v.Valuation
	// Reference cost = the latest purchase cost (most recent batch's cost_price,
	// per base unit, global). Drives the markup/margin readout in the product
	// form. Detail-only; 0 when the product has no batch yet.
	var refCost *int64
	if err := m.db.WithContext(ctx).
		Model(&model.Batch{}).
		Where("product_id = ?", med.ID).
		Order("received_at DESC, created_at DESC").
		Limit(1).
		Select("cost_price").
		Scan(&refCost).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if refCost != nil {
		out.ReferenceCost = *refCost
	}
	if err := m.attachUnits(ctx, []*inventoryifacev1.Product{out}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.GetProductResponse{Product: out}), nil
}

func (m *Products) CreateProduct(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.CreateProductRequest],
) (*connect.Response[inventoryifacev1.CreateProductResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}

	sku := strings.TrimSpace(req.Msg.Sku)
	name := strings.TrimSpace(req.Msg.Name)
	unit := strings.TrimSpace(req.Msg.Unit)
	if sku == "" || name == "" || unit == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("sku, name, unit required"))
	}
	if req.Msg.UnitPrice < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unit_price must be >= 0"))
	}

	med := model.Product{
		SKU:                  sku,
		Name:                 name,
		Unit:                 unit,
		UnitPrice:            req.Msg.UnitPrice,
		PrescriptionRequired: req.Msg.PrescriptionRequired,
		Active:               true,
	}

	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&med).Error; err != nil {
			return fmt.Errorf("create product: %w", err)
		}
		price := model.ProductPrice{
			ProductID:     med.ID,
			UnitPrice:     med.UnitPrice,
			EffectiveFrom: time.Now(),
			ChangedBy:     caller.UserID,
		}
		if err := tx.Create(&price).Error; err != nil {
			return fmt.Errorf("create initial price: %w", err)
		}
		// Base unit (factor 1) + any additional units supplied.
		if err := syncProductUnits(tx, &med, req.Msg.Units, caller.UserID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		var ce *connect.Error
		if errors.As(err, &ce) {
			return nil, err // unit validation error — keep its code
		}
		return nil, connect.NewError(connect.CodeAlreadyExists, err) // likely dup SKU
	}
	out := productToProto(&med)
	if err := m.attachUnits(ctx, []*inventoryifacev1.Product{out}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.CreateProductResponse{Product: out}), nil
}

func (m *Products) UpdateProduct(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UpdateProductRequest],
) (*connect.Response[inventoryifacev1.UpdateProductResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}

	med, err := m.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Msg.Name)
	unit := strings.TrimSpace(req.Msg.Unit)
	if name == "" || unit == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name and unit required"))
	}
	if req.Msg.UnitPrice < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unit_price must be >= 0"))
	}

	priceChanged := req.Msg.UnitPrice != med.UnitPrice

	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the product row so concurrent price edits serialize — otherwise the
		// close-open-row + insert-new-open-row price-version sequence (here and in
		// syncProductUnits/recordUnitPrice) can collide on the *_open_idx partial
		// unique index and fail spuriously.
		if err := rowLock(tx).
			Where("id = ?", med.ID).First(&model.Product{}).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"name":                  name,
			"unit":                  unit,
			"prescription_required": req.Msg.PrescriptionRequired,
		}
		if priceChanged {
			updates["unit_price"] = req.Msg.UnitPrice
		}
		if err := tx.Model(med).Updates(updates).Error; err != nil {
			return err
		}

		if priceChanged {
			now := time.Now()
			// Close the current open price row.
			if err := tx.Model(&model.ProductPrice{}).
				Where("product_id = ? AND effective_to IS NULL", med.ID).
				Update("effective_to", now).Error; err != nil {
				return fmt.Errorf("close current price: %w", err)
			}
			// Insert the new open row.
			newPrice := model.ProductPrice{
				ProductID:     med.ID,
				UnitPrice:     req.Msg.UnitPrice,
				EffectiveFrom: now,
				ChangedBy:     caller.UserID,
			}
			if err := tx.Create(&newPrice).Error; err != nil {
				return fmt.Errorf("insert new price: %w", err)
			}
		}

		// Sync units against the new base name/price.
		med.Unit = unit
		med.UnitPrice = req.Msg.UnitPrice
		if err := syncProductUnits(tx, med, req.Msg.Units, caller.UserID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		var ce *connect.Error
		if errors.As(err, &ce) {
			return nil, err
		}
		return nil, connect.NewError(connect.CodeAborted, err)
	}

	// Refresh from DB so response reflects the new state.
	med, err = m.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	out := productToProto(med)
	if err := m.attachUnits(ctx, []*inventoryifacev1.Product{out}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.UpdateProductResponse{Product: out}), nil
}

func (m *Products) ArchiveProduct(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ArchiveProductRequest],
) (*connect.Response[inventoryifacev1.ArchiveProductResponse], error) {
	med, err := m.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := m.db.WithContext(ctx).Model(med).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	med.Active = false
	return connect.NewResponse(&inventoryifacev1.ArchiveProductResponse{Product: productToProto(med)}), nil
}

func (m *Products) UnarchiveProduct(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UnarchiveProductRequest],
) (*connect.Response[inventoryifacev1.UnarchiveProductResponse], error) {
	med, err := m.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := m.db.WithContext(ctx).Model(med).Update("active", true).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	med.Active = true
	return connect.NewResponse(&inventoryifacev1.UnarchiveProductResponse{Product: productToProto(med)}), nil
}

func (m *Products) SearchProducts(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.SearchProductsRequest],
) (*connect.Response[inventoryifacev1.SearchProductsResponse], error) {
	query := strings.TrimSpace(req.Msg.Query)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := m.db.WithContext(ctx).Order("name").Limit(limit)
	if !req.Msg.IncludeInactive {
		q = q.Where("active = ?", true)
	}
	if query != "" {
		pattern := "%" + query + "%"
		q = q.Where("name "+likeOp(q)+" ? OR sku "+likeOp(q)+" ?", pattern, pattern)
	}
	var rows []model.Product
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Product, 0, len(rows))
	for i := range rows {
		out = append(out, productToProto(&rows[i]))
	}
	if err := m.attachUnits(ctx, out); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.SearchProductsResponse{Products: out}), nil
}

// ResolveProducts returns minimal display refs for a set of ids. Unknown ids
// are omitted; empty input returns an empty list. No enrich, no preload.
func (m *Products) ResolveProducts(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ResolveProductsRequest],
) (*connect.Response[inventoryifacev1.ResolveProductsResponse], error) {
	ids := dedupeIDs(req.Msg.Ids)
	if len(ids) == 0 {
		return connect.NewResponse(&inventoryifacev1.ResolveProductsResponse{}), nil
	}
	type row struct {
		ID   string `gorm:"column:id"`
		Name string `gorm:"column:name"`
		SKU  string `gorm:"column:sku"`
	}
	var rows []row
	if err := m.db.WithContext(ctx).
		Model(&model.Product{}).
		Select("id, name, sku").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.ProductRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &inventoryifacev1.ProductRef{Id: r.ID, Name: r.Name, Sku: r.SKU})
	}
	return connect.NewResponse(&inventoryifacev1.ResolveProductsResponse{Products: out}), nil
}

func (m *Products) ListProductPrices(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductPricesRequest],
) (*connect.Response[inventoryifacev1.ListProductPricesResponse], error) {
	if req.Msg.ProductId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_id required"))
	}
	var rows []model.ProductPrice
	err := m.db.WithContext(ctx).
		Where("product_id = ?", req.Msg.ProductId).
		Order("effective_from DESC").
		Find(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.ProductPrice, 0, len(rows))
	for _, r := range rows {
		out = append(out, productPriceToProto(&r))
	}
	return connect.NewResponse(&inventoryifacev1.ListProductPricesResponse{Prices: out}), nil
}

// ListProductUnitPrices returns the per-unit sell-price history for a product,
// joined to the unit name and ordered base-first then newest-first.
func (m *Products) ListProductUnitPrices(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductUnitPricesRequest],
) (*connect.Response[inventoryifacev1.ListProductUnitPricesResponse], error) {
	if req.Msg.ProductId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_id required"))
	}
	type row struct {
		ID            string     `gorm:"column:id"`
		ProductUnitID string     `gorm:"column:product_unit_id"`
		UnitName      string     `gorm:"column:unit_name"`
		UnitSellPrice int64      `gorm:"column:unit_sell_price"`
		EffectiveFrom time.Time  `gorm:"column:effective_from"`
		EffectiveTo   *time.Time `gorm:"column:effective_to"`
		ChangedBy     *string    `gorm:"column:changed_by"`
	}
	var rows []row
	err := m.db.WithContext(ctx).
		Table("product_unit_prices mup").
		Select(`mup.id, mup.product_unit_id, mu.name AS unit_name, mup.unit_sell_price,
		        mup.effective_from, mup.effective_to, mup.changed_by`).
		Joins("JOIN product_units mu ON mu.id = mup.product_unit_id").
		Where("mu.product_id = ?", req.Msg.ProductId).
		Order("mu.is_base DESC, mu.factor ASC, mup.effective_from DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.ProductUnitPrice, 0, len(rows))
	for _, r := range rows {
		p := &inventoryifacev1.ProductUnitPrice{
			Id:            r.ID,
			ProductUnitId: r.ProductUnitID,
			UnitName:      r.UnitName,
			UnitSellPrice: r.UnitSellPrice,
			EffectiveFrom: r.EffectiveFrom.Unix(),
		}
		if r.EffectiveTo != nil {
			p.EffectiveTo = r.EffectiveTo.Unix()
		}
		if r.ChangedBy != nil {
			p.ChangedBy = *r.ChangedBy
		}
		out = append(out, p)
	}
	return connect.NewResponse(&inventoryifacev1.ListProductUnitPricesResponse{Prices: out}), nil
}

// ListLowStock returns active products whose ready_stock in the caller's
// active warehouse is at or below the configured low-stock threshold. Single
// query: GROUP BY product, HAVING SUM(qty in this warehouse) <= threshold,
// ordered by ready ASC then name ASC. Capped at 100 (low-stock items should be
// few; the bell dropdown scrolls).
func (m *Products) ListLowStock(
	ctx context.Context,
	_ *connect.Request[inventoryifacev1.ListLowStockRequest],
) (*connect.Response[inventoryifacev1.ListLowStockResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := resolveWarehouse(ctx, m.db, caller)
	if err != nil {
		return nil, err
	}
	threshold, err := getLowStockThreshold(ctx, m.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	type lowRow struct {
		model.Product
		Ready int64 `gorm:"column:ready"`
	}
	var rows []lowRow
	if err := m.db.WithContext(ctx).
		Table("products AS m").
		Select("m.*, COALESCE(SUM(sm.qty), 0) AS ready").
		Joins("LEFT JOIN batches AS b ON b.product_id = m.id").
		Joins("LEFT JOIN stock_movements AS sm ON sm.batch_id = b.id AND sm.warehouse_id = ?", warehouseID).
		Where("m.active = ?", true).
		Group("m.id").
		Having("COALESCE(SUM(sm.qty), 0) <= ?", threshold).
		Order("ready ASC, m.name ASC").
		Limit(100).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*inventoryifacev1.Product, 0, len(rows))
	for i := range rows {
		p := productToProto(&rows[i].Product)
		p.ReadyStock = rows[i].Ready
		out = append(out, p)
	}
	if err := m.attachUnits(ctx, out); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.ListLowStockResponse{
		Products:  out,
		Threshold: threshold,
		Total:     int32(len(out)),
	}), nil
}

func (m *Products) load(ctx context.Context, id string) (*model.Product, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var med model.Product
	err := m.db.WithContext(ctx).Where("id = ?", id).First(&med).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("product %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &med, nil
}

func productToProto(m *model.Product) *inventoryifacev1.Product {
	return &inventoryifacev1.Product{
		Id:                   m.ID,
		Sku:                  m.SKU,
		Name:                 m.Name,
		Unit:                 m.Unit,
		UnitPrice:            m.UnitPrice,
		PrescriptionRequired: m.PrescriptionRequired,
		Active:               m.Active,
		CreatedAt:            m.CreatedAt.Unix(),
	}
}

func productUnitToProto(u *model.ProductUnit) *inventoryifacev1.ProductUnit {
	return &inventoryifacev1.ProductUnit{
		Id:          u.ID,
		ProductId:   u.ProductID,
		Name:        u.Name,
		Factor:      u.Factor,
		IsBase:      u.IsBase,
		SellPrice:   u.SellPrice,
		Sellable:    u.Sellable,
		Purchasable: u.Purchasable,
		SortOrder:   int32(u.SortOrder),
		Active:      u.Active,
	}
}

// attachUnits batch-loads each product's active units (base first, then by
// factor) and sets them on the protos. No N+1.
func (m *Products) attachUnits(ctx context.Context, meds []*inventoryifacev1.Product) error {
	if len(meds) == 0 {
		return nil
	}
	ids := make([]string, 0, len(meds))
	for _, md := range meds {
		ids = append(ids, md.Id)
	}
	var rows []model.ProductUnit
	if err := m.db.WithContext(ctx).
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

// syncProductUnits upserts a product's units inside a tx: the base unit is
// derived from med.Unit/med.UnitPrice (factor 1); `inputs` are the larger
// (non-base) units. Non-base units absent from `inputs` are deactivated.
func syncProductUnits(tx *gorm.DB, med *model.Product, inputs []*inventoryifacev1.ProductUnitInput, changedBy string) error {
	// Upsert the base unit.
	var base model.ProductUnit
	err := tx.Where("product_id = ? AND is_base", med.ID).First(&base).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		base = model.ProductUnit{
			ProductID: med.ID, Name: med.Unit, Factor: 1, IsBase: true,
			SellPrice: med.UnitPrice, Sellable: true, Purchasable: true, Active: true,
		}
		if err := tx.Create(&base).Error; err != nil {
			return err
		}
		if err := recordUnitPrice(tx, base.ID, med.UnitPrice, changedBy); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if err := tx.Model(&model.ProductUnit{}).Where("id = ?", base.ID).
			Updates(map[string]any{"name": med.Unit, "sell_price": med.UnitPrice, "active": true}).Error; err != nil {
			return err
		}
		if base.SellPrice != med.UnitPrice {
			if err := recordUnitPrice(tx, base.ID, med.UnitPrice, changedBy); err != nil {
				return err
			}
		}
	}

	keptIDs := []string{base.ID}
	for _, in := range inputs {
		name := strings.TrimSpace(in.Name)
		if name == "" {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("unit name required"))
		}
		if in.Factor <= 1 {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unit %q factor must be > 1", name))
		}
		if in.SellPrice < 0 {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("sell_price must be >= 0"))
		}
		if in.Id != "" {
			var existing model.ProductUnit
			if err := tx.Where("id = ? AND product_id = ?", in.Id, med.ID).First(&existing).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.ProductUnit{}).
				Where("id = ? AND product_id = ?", in.Id, med.ID).
				Updates(map[string]any{
					"name": name, "factor": in.Factor, "sell_price": in.SellPrice,
					"sellable": in.Sellable, "purchasable": in.Purchasable,
					"sort_order": int(in.SortOrder), "active": true,
				}).Error; err != nil {
				return err
			}
			if existing.SellPrice != in.SellPrice {
				if err := recordUnitPrice(tx, in.Id, in.SellPrice, changedBy); err != nil {
					return err
				}
			}
			keptIDs = append(keptIDs, in.Id)
		} else {
			row := model.ProductUnit{
				ProductID: med.ID, Name: name, Factor: in.Factor, IsBase: false,
				SellPrice: in.SellPrice, Sellable: in.Sellable, Purchasable: in.Purchasable,
				SortOrder: int(in.SortOrder), Active: true,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			if err := recordUnitPrice(tx, row.ID, in.SellPrice, changedBy); err != nil {
				return err
			}
			keptIDs = append(keptIDs, row.ID)
		}
	}
	// Deactivate non-base units that were removed from the set.
	return tx.Model(&model.ProductUnit{}).
		Where("product_id = ? AND is_base = false AND id NOT IN ?", med.ID, keptIDs).
		Update("active", false).Error
}

// recordUnitPrice closes a unit's open price row (if any) and inserts a new open
// row, mirroring the product_prices versioning for the base price.
func recordUnitPrice(tx *gorm.DB, unitID string, newPrice int64, changedBy string) error {
	now := time.Now()
	if err := tx.Model(&model.ProductUnitPrice{}).
		Where("product_unit_id = ? AND effective_to IS NULL", unitID).
		Update("effective_to", now).Error; err != nil {
		return err
	}
	row := model.ProductUnitPrice{
		ProductUnitID: unitID,
		UnitSellPrice: newPrice,
		EffectiveFrom: now,
	}
	if changedBy != "" {
		row.ChangedBy = &changedBy
	}
	return tx.Create(&row).Error
}

func productPriceToProto(p *model.ProductPrice) *inventoryifacev1.ProductPrice {
	out := &inventoryifacev1.ProductPrice{
		Id:            p.ID,
		ProductId:     p.ProductID,
		UnitPrice:     p.UnitPrice,
		EffectiveFrom: p.EffectiveFrom.Unix(),
		ChangedBy:     p.ChangedBy,
	}
	if p.EffectiveTo != nil {
		out.EffectiveTo = p.EffectiveTo.Unix()
	}
	return out
}

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/printer"
)

const (
	saleStatusDraft     = "DRAFT"
	saleStatusCompleted = "COMPLETED"
	saleStatusVoided    = "VOIDED"

	paymentCash    = "CASH"
	paymentNonCash = "NON_CASH"

	movementTypeSale = "SALE"
)

type Sales struct {
	db      *gorm.DB
	printer config.Printer
}

func NewSales(db *gorm.DB, printerCfg config.Printer) *Sales {
	return &Sales{db: db, printer: printerCfg}
}

// ---------- Lifecycle ----------

func (s *Sales) StartSale(
	ctx context.Context,
	_ *connect.Request[posifacev1.StartSaleRequest],
) (*connect.Response[posifacev1.StartSaleResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := resolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	sale := model.Sale{
		CashierUserID: caller.UserID,
		Status:        saleStatusDraft,
		WarehouseID:   &warehouseID,
	}
	if err := s.db.WithContext(ctx).Create(&sale).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out, err := s.loadFull(ctx, sale.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.StartSaleResponse{Sale: saleToProto(out)}), nil
}

func (s *Sales) GetSale(
	ctx context.Context,
	req *connect.Request[posifacev1.GetSaleRequest],
) (*connect.Response[posifacev1.GetSaleResponse], error) {
	sale, err := s.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.GetSaleResponse{Sale: saleToProto(sale)}), nil
}

func (s *Sales) ListSales(
	ctx context.Context,
	req *connect.Request[posifacev1.ListSalesRequest],
) (*connect.Response[posifacev1.ListSalesResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := resolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	limit, offset := normPage(req.Msg.Limit, req.Msg.Offset)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		return s.applySaleFilters(q, warehouseID, req.Msg.FromUnix, req.Msg.ToUnix, req.Msg.Status, req.Msg.Query)
	}

	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Sale{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.Sale
	if err := applyFilters(s.db.WithContext(ctx).Preload("Items")).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*posifacev1.Sale, 0, len(rows))
	for i := range rows {
		out = append(out, saleToProto(&rows[i]))
	}
	if err := s.enrichSaleNames(ctx, out); err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.ListSalesResponse{
		Sales: out,
		Total: int32(total),
	}), nil
}

// applySaleFilters applies the order-history filters (date range, status,
// free-text search) shared by ListSales and GetSalesSummary so the paginated
// list and its summary always agree. The caller's query root must be `sales`
// (the unqualified columns + `id IN (...)` resolve against it).
func (s *Sales) applySaleFilters(
	q *gorm.DB,
	warehouseID string,
	fromUnix, toUnix int64,
	status posifacev1.SaleStatus,
	query string,
) *gorm.DB {
	if warehouseID != "" {
		q = q.Where("warehouse_id = ?", warehouseID)
	}
	if fromUnix > 0 {
		q = q.Where("created_at >= ?", time.Unix(fromUnix, 0))
	}
	if toUnix > 0 {
		q = q.Where("created_at < ?", time.Unix(toUnix, 0))
	}
	if statusStr := saleStatusToString(status); statusStr != "" {
		q = q.Where("status = ?", statusStr)
	} else {
		// "All" in order history means finalized orders only — in-progress
		// carts (DRAFT) are never shown in history or its summary.
		q = q.Where("status <> ?", saleStatusDraft)
	}
	if qstr := strings.TrimSpace(query); qstr != "" {
		pattern := "%" + qstr + "%"
		sub := s.db.Table("sales AS s").Select("s.id").
			Joins("LEFT JOIN customers c ON c.id = s.customer_id").
			Joins("LEFT JOIN sale_items si ON si.sale_id = s.id").
			Joins("LEFT JOIN products m ON m.id = si.product_id").
			Where("s.sale_no ILIKE ? OR c.name ILIKE ? OR m.name ILIKE ?", pattern, pattern, pattern)
		q = q.Where("id IN (?)", sub)
	}
	return q
}

// GetSalesSummary aggregates over the SAME filters as ListSales across ALL
// matching rows (not one page). The list stays server-paginated via ListSales;
// this is a separate server-side aggregate (never a client-side sum of a page).
func (s *Sales) GetSalesSummary(
	ctx context.Context,
	req *connect.Request[posifacev1.GetSalesSummaryRequest],
) (*connect.Response[posifacev1.GetSalesSummaryResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := resolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	scope := func() *gorm.DB {
		return s.applySaleFilters(s.db.WithContext(ctx).Model(&model.Sale{}),
			warehouseID, req.Msg.FromUnix, req.Msg.ToUnix, req.Msg.Status, req.Msg.Query)
	}

	var saleCount int64
	if err := scope().Count(&saleCount).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var revenue int64
	if err := scope().Select("COALESCE(SUM(total), 0)").Scan(&revenue).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// items_sold = SUM(qty) over sale_items whose sale matches the same filters.
	idSub := s.applySaleFilters(s.db.WithContext(ctx).Model(&model.Sale{}).Select("id"),
		warehouseID, req.Msg.FromUnix, req.Msg.ToUnix, req.Msg.Status, req.Msg.Query)
	var itemsSold int64
	if err := s.db.WithContext(ctx).
		Table("sale_items").
		Where("sale_id IN (?)", idSub).
		Select("COALESCE(SUM(base_qty), 0)").
		Scan(&itemsSold).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&posifacev1.GetSalesSummaryResponse{
		SaleCount: saleCount,
		ItemsSold: itemsSold,
		Revenue:   revenue,
	}), nil
}

// enrichSaleNames denormalizes customer + product names onto a page of sales
// for the order-history list (two batched queries, no N+1).
func (s *Sales) enrichSaleNames(ctx context.Context, sales []*posifacev1.Sale) error {
	if len(sales) == 0 {
		return nil
	}
	custIDSet := map[string]struct{}{}
	medIDSet := map[string]struct{}{}
	for _, sl := range sales {
		if sl.CustomerId != "" {
			custIDSet[sl.CustomerId] = struct{}{}
		}
		for _, it := range sl.Items {
			if it.ProductId != "" {
				medIDSet[it.ProductId] = struct{}{}
			}
		}
	}
	nameByID := func(table string, idset map[string]struct{}) (map[string]string, error) {
		out := map[string]string{}
		if len(idset) == 0 {
			return out, nil
		}
		ids := make([]string, 0, len(idset))
		for id := range idset {
			ids = append(ids, id)
		}
		type row struct {
			ID   string `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		var rows []row
		if err := s.db.WithContext(ctx).Table(table).Select("id, name").
			Where("id IN ?", ids).Scan(&rows).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, r := range rows {
			out[r.ID] = r.Name
		}
		return out, nil
	}
	custNames, err := nameByID("customers", custIDSet)
	if err != nil {
		return err
	}
	medNames, err := nameByID("products", medIDSet)
	if err != nil {
		return err
	}
	for _, sl := range sales {
		sl.CustomerName = custNames[sl.CustomerId]
		for _, it := range sl.Items {
			it.ProductName = medNames[it.ProductId]
		}
	}
	return nil
}

// ---------- Item ops ----------

func (s *Sales) AddItem(
	ctx context.Context,
	req *connect.Request[posifacev1.AddItemRequest],
) (*connect.Response[posifacev1.AddItemResponse], error) {
	if req.Msg.Qty <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("qty must be > 0"))
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}

		// Look up product to snapshot the current price.
		var med model.Product
		if err := tx.Where("id = ?", req.Msg.ProductId).First(&med).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, fmt.Errorf("product %s not found", req.Msg.ProductId))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if !med.Active {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("product is archived"))
		}

		// Resolve the selling unit (box/strip/tablet, …) — default base.
		unit, err := resolveSellUnit(tx, med.ID, req.Msg.ProductUnitId)
		if err != nil {
			return err
		}

		// Look up the existing line for this product + SAME unit (a box line and a
		// tablet line of the same product are distinct).
		var existing model.SaleItem
		findErr := tx.Where("sale_id = ? AND product_id = ? AND product_unit_id = ?", sale.ID, med.ID, unit.ID).
			First(&existing).Error
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return connect.NewError(connect.CodeInternal, findErr)
		}
		newUnitQty := req.Msg.Qty
		if findErr == nil {
			newUnitQty = existing.Qty + req.Msg.Qty
		}
		newBaseQty := newUnitQty * int32(unit.Factor)

		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			unitID := unit.ID
			item := model.SaleItem{
				SaleID:            sale.ID,
				ProductID:        med.ID,
				ProductUnitID:    &unitID,
				UnitName:          unit.Name,
				UnitFactor:        unit.Factor,
				Qty:               req.Msg.Qty,
				BaseQty:           req.Msg.Qty * int32(unit.Factor),
				UnitPriceSnapshot: unit.SellPrice,
			}
			item.LineTotal = computeLineTotal(item.Qty, item.UnitPriceSnapshot, item.LineDiscount)
			if err := tx.Create(&item).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		} else {
			existing.Qty = newUnitQty
			existing.BaseQty = newBaseQty
			existing.UnitName = unit.Name
			existing.UnitFactor = unit.Factor
			existing.UnitPriceSnapshot = unit.SellPrice
			existing.LineTotal = computeLineTotal(existing.Qty, existing.UnitPriceSnapshot, existing.LineDiscount)
			if err := tx.Save(&existing).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		return recomputeSaleTotals(tx, sale.ID)
	})
	if err != nil {
		return nil, asConnectErr(err)
	}

	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.AddItemResponse{Sale: saleToProto(sale)}), nil
}

func (s *Sales) SetItemQuantity(
	ctx context.Context,
	req *connect.Request[posifacev1.SetItemQuantityRequest],
) (*connect.Response[posifacev1.SetItemQuantityResponse], error) {
	if req.Msg.Qty <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("qty must be > 0"))
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		var item model.SaleItem
		if err := tx.Where("id = ? AND sale_id = ?", req.Msg.ItemId, sale.ID).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("item not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		factor := item.UnitFactor
		if factor < 1 {
			factor = 1
		}
		item.Qty = req.Msg.Qty
		item.BaseQty = req.Msg.Qty * int32(factor)
		item.LineTotal = computeLineTotal(item.Qty, item.UnitPriceSnapshot, item.LineDiscount)
		if err := tx.Save(&item).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return recomputeSaleTotals(tx, sale.ID)
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.SetItemQuantityResponse{Sale: saleToProto(sale)}), nil
}

func (s *Sales) RemoveItem(
	ctx context.Context,
	req *connect.Request[posifacev1.RemoveItemRequest],
) (*connect.Response[posifacev1.RemoveItemResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		res := tx.Where("id = ? AND sale_id = ?", req.Msg.ItemId, sale.ID).Delete(&model.SaleItem{})
		if res.Error != nil {
			return connect.NewError(connect.CodeInternal, res.Error)
		}
		if res.RowsAffected == 0 {
			return connect.NewError(connect.CodeNotFound, errors.New("item not found"))
		}
		return recomputeSaleTotals(tx, sale.ID)
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.RemoveItemResponse{Sale: saleToProto(sale)}), nil
}

func (s *Sales) SetSaleCustomer(
	ctx context.Context,
	req *connect.Request[posifacev1.SetSaleCustomerRequest],
) (*connect.Response[posifacev1.SetSaleCustomerResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		updates := map[string]any{"customer_id": nil}
		if req.Msg.CustomerId != "" {
			updates["customer_id"] = req.Msg.CustomerId
		}
		return tx.Model(sale).Updates(updates).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.SetSaleCustomerResponse{Sale: saleToProto(sale)}), nil
}

// ---------- Complete (FEFO + stock movements + sale numbering) ----------

func (s *Sales) CompleteSale(
	ctx context.Context,
	req *connect.Request[posifacev1.CompleteSaleRequest],
) (*connect.Response[posifacev1.CompleteSaleResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	paymentStr, err := paymentSourceToString(req.Msg.PaymentSource)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}

		var items []model.SaleItem
		if err := tx.Where("sale_id = ?", sale.ID).Order("created_at").Find(&items).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if len(items) == 0 {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("cart is empty"))
		}

		// Cash requires paid_amount >= total. Non-cash settles externally
		// (card / QRIS / transfer), so any paid_amount is accepted.
		if paymentStr == paymentCash && req.Msg.PaidAmount < sale.Total {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("paid_amount less than total"))
		}

		now := time.Now()

		// FEFO consumes stock from the sale's warehouse only.
		saleWh := deref(sale.WarehouseID)

		// Lock every lot of the cart's products FOR UPDATE (deterministic id
		// order) BEFORE reading availability, so two concurrent CompleteSale (or a
		// transfer / adjustment) for the same lot serialize and can't both pass the
		// availability check and oversell (the ledger SUM under READ COMMITTED
		// otherwise misses the other tx's uncommitted movement).
		medSet := make(map[string]struct{}, len(items))
		medIDs := make([]string, 0, len(items))
		for i := range items {
			if _, ok := medSet[items[i].ProductID]; ok {
				continue
			}
			medSet[items[i].ProductID] = struct{}{}
			medIDs = append(medIDs, items[i].ProductID)
		}
		if err := lockBatchesByProduct(tx, medIDs); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		// For each line: consume its BASE-unit quantity across FEFO batches in the
		// sale's warehouse. The sale_item stays one row per selling line; the
		// per-batch breakdown lives on the SALE stock_movements (each linked to the
		// line via sale_item_id) — which is also the COGS source.
		for i := range items {
			item := items[i]
			needed := item.BaseQty
			if needed <= 0 {
				needed = item.Qty // back-compat for any rows created before UOM
			}

			// Available batches for the product, ordered by expiry ASC (FEFO).
			var batches []model.Batch
			if err := tx.Where("product_id = ?", item.ProductID).
				Order("expiry_date ASC").
				Find(&batches).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			for _, b := range batches {
				if needed <= 0 {
					break
				}
				// Per-warehouse available qty — only this warehouse's stock.
				var avail int64
				if err := tx.Model(&model.StockMovement{}).
					Where("batch_id = ? AND warehouse_id = ?", b.ID, saleWh).
					Select("COALESCE(SUM(qty), 0)").
					Scan(&avail).Error; err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}
				if avail <= 0 {
					continue
				}
				take := int64(needed)
				if take > avail {
					take = avail
				}

				// SALE movement (negative base qty), linked to the selling line.
				saleItemID := item.ID
				mv := model.StockMovement{
					BatchID:     b.ID,
					Qty:         -int32(take),
					Type:        movementTypeSale,
					Reason:      "POS sale",
					UserID:      caller.UserID,
					SaleItemID:  &saleItemID,
					WarehouseID: saleWh,
				}
				if err := tx.Create(&mv).Error; err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}

				needed -= int32(take)
			}

			if needed > 0 {
				// Insufficient base stock in this warehouse — abort the whole tx.
				return connect.NewError(connect.CodeFailedPrecondition,
					fmt.Errorf("insufficient stock for product %s (%d base units short)", item.ProductID, needed))
			}
		}

		// Recompute totals from the now-allocated sale_items.
		if err := recomputeSaleTotals(tx, sale.ID); err != nil {
			return err
		}

		// Assign per-year sale_no.
		saleNo, err := assignSaleNo(tx, now)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		// Finalize the sale row.
		updates := map[string]any{
			"sale_no":        saleNo,
			"payment_source": paymentStr,
			"paid_amount":    req.Msg.PaidAmount,
			"status":         saleStatusCompleted,
			"completed_at":   now,
		}
		if err := tx.Model(sale).Updates(updates).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		sale.Status = saleStatusCompleted
		sale.CompletedAt = &now
		sale.SaleNo = &saleNo
		sale.PaymentSource = &paymentStr
		sale.PaidAmount = req.Msg.PaidAmount

		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}

	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.CompleteSaleResponse{Sale: saleToProto(sale)}), nil
}

func (s *Sales) VoidSale(
	ctx context.Context,
	req *connect.Request[posifacev1.VoidSaleRequest],
) (*connect.Response[posifacev1.VoidSaleResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sale model.Sale
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", req.Msg.SaleId).First(&sale).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("sale not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if sale.Status != saleStatusDraft {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("only draft sales can be voided; this one is %s", sale.Status))
		}
		return tx.Model(&sale).Update("status", saleStatusVoided).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.VoidSaleResponse{Sale: saleToProto(sale)}), nil
}

// DiscardSale hard-deletes a DRAFT sale + its items so an abandoned cart leaves
// no trace (not even a VOIDED row). Safe: a DRAFT has no stock_movements (stock
// is consumed only at CompleteSale) and no sale_no, so nothing references it.
func (s *Sales) DiscardSale(
	ctx context.Context,
	req *connect.Request[posifacev1.DiscardSaleRequest],
) (*connect.Response[posifacev1.DiscardSaleResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sale model.Sale
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", req.Msg.SaleId).First(&sale).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("sale not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if sale.Status != saleStatusDraft {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("only draft sales can be discarded; this one is %s", sale.Status))
		}
		if err := tx.Where("sale_id = ?", sale.ID).Delete(&model.SaleItem{}).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := tx.Delete(&sale).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	return connect.NewResponse(&posifacev1.DiscardSaleResponse{}), nil
}

// ---------- Snapshot ----------

func (s *Sales) GetTodaySnapshot(
	ctx context.Context,
	req *connect.Request[posifacev1.GetTodaySnapshotRequest],
) (*connect.Response[posifacev1.GetTodaySnapshotResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, werr := resolveWarehouse(ctx, s.db, caller)
	if werr != nil {
		return nil, connect.NewError(connect.CodeInternal, werr)
	}

	// Optional cashier filter — non-OWNER callers may only request their own.
	cashierID := req.Msg.CashierUserId
	if cashierID != "" && caller.Role != "OWNER" && cashierID != caller.UserID {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("can only request own snapshot"))
	}

	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// applySaleFilters mirrors the unified-filter pattern in ListSales — adds
	// status + warehouse + day + optional cashier predicates to a query whose
	// FROM has `sales` aliased (caller passes the alias prefix, e.g. "" for
	// Model(&Sale{}) or "s." for a JOIN).
	applySaleFilters := func(q *gorm.DB, alias string) *gorm.DB {
		q = q.Where(alias+"status = ?", saleStatusCompleted).
			Where(alias+"warehouse_id = ?", warehouseID).
			Where(alias+"completed_at >= ?", dayStart)
		if cashierID != "" {
			q = q.Where(alias+"cashier_user_id = ?", cashierID)
		}
		return q
	}

	var revenue int64
	if err := applySaleFilters(s.db.WithContext(ctx).Model(&model.Sale{}), "").
		Select("COALESCE(SUM(total), 0)").
		Scan(&revenue).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var saleCount int64
	if err := applySaleFilters(s.db.WithContext(ctx).Model(&model.Sale{}), "").
		Count(&saleCount).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var itemsSold int64
	if err := applySaleFilters(
		s.db.WithContext(ctx).Table("sale_items si").Joins("JOIN sales s ON s.id = si.sale_id"),
		"s.",
	).Select("COALESCE(SUM(si.base_qty), 0)").
		Scan(&itemsSold).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	type topRow struct {
		ProductID string
		Qty        int64
	}
	var top topRow
	_ = applySaleFilters(
		s.db.WithContext(ctx).Table("sale_items si").Joins("JOIN sales s ON s.id = si.sale_id"),
		"s.",
	).Select("si.product_id AS product_id, SUM(si.base_qty) AS qty").
		Group("si.product_id").
		Order("qty DESC").
		Limit(1).
		Scan(&top).Error

	var lastSaleUnix int64
	if err := applySaleFilters(s.db.WithContext(ctx).Model(&model.Sale{}), "").
		Select("COALESCE(EXTRACT(EPOCH FROM MAX(completed_at))::bigint, 0)").
		Scan(&lastSaleUnix).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&posifacev1.GetTodaySnapshotResponse{
		Revenue:        revenue,
		SaleCount:      saleCount,
		ItemsSold:      itemsSold,
		TopProductId:  top.ProductID,
		TopProductQty: top.Qty,
		LastSaleUnix:   lastSaleUnix,
	}), nil
}

// ---------- Print ----------

func (s *Sales) PrintReceipt(
	ctx context.Context,
	req *connect.Request[posifacev1.PrintReceiptRequest],
) (*connect.Response[posifacev1.PrintReceiptResponse], error) {
	if !s.printer.Enabled {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("printer is not configured (set printer.enabled in config.yaml)"))
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	if sale.Status != saleStatusCompleted {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("only completed sales can be printed"))
	}

	// Resolve product names.
	medIDs := make([]string, 0, len(sale.Items))
	for _, it := range sale.Items {
		medIDs = append(medIDs, it.ProductID)
	}
	var meds []model.Product
	if err := s.db.WithContext(ctx).Where("id IN ?", medIDs).Find(&meds).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	medName := make(map[string]string, len(meds))
	for _, m := range meds {
		medName[m.ID] = m.Name
	}

	// Resolve cashier name.
	var cashier model.User
	cashierName := ""
	if err := s.db.WithContext(ctx).Where("id = ?", sale.CashierUserID).First(&cashier).Error; err == nil {
		cashierName = cashier.Name
		if cashierName == "" {
			cashierName = cashier.Email
		}
	}

	// Resolve customer name (optional).
	customerName := ""
	if sale.CustomerID != nil && *sale.CustomerID != "" {
		var customer model.Customer
		if err := s.db.WithContext(ctx).Where("id = ?", *sale.CustomerID).First(&customer).Error; err == nil {
			customerName = customer.Name
		}
	}

	// Build the renderer Receipt.
	lines := make([]printer.ReceiptLine, 0, len(sale.Items))
	for _, it := range sale.Items {
		lines = append(lines, printer.ReceiptLine{
			Qty:       it.Qty,
			UnitName:  it.UnitName,
			Name:      medName[it.ProductID],
			LineTotal: it.LineTotal,
		})
	}
	completedAt := time.Time{}
	if sale.CompletedAt != nil {
		completedAt = *sale.CompletedAt
	}
	saleNo := ""
	if sale.SaleNo != nil {
		saleNo = *sale.SaleNo
	}
	paymentStr := ""
	if sale.PaymentSource != nil {
		paymentStr = *sale.PaymentSource
	}
	change := int64(0)
	if paymentStr == paymentCash && sale.PaidAmount > sale.Total {
		change = sale.PaidAmount - sale.Total
	}

	receipt := printer.Receipt{
		SaleNo:      saleNo,
		CompletedAt: completedAt,
		Cashier:     cashierName,
		Customer:    customerName,
		Items:       lines,
		Subtotal:    sale.Subtotal,
		Total:       sale.Total,
		Paid:        sale.PaidAmount,
		Payment:     paymentStr,
		Change:      change,
	}
	settings := printer.Settings{
		Width:      s.printer.Width,
		Header:     s.printer.Header,
		Footer:     s.printer.Footer,
		OpenDrawer: s.printer.OpenDrawer,
	}
	payload := printer.Render(receipt, settings)

	if err := printer.DispatchTCP(s.printer.Address, payload, s.printer.Timeout); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&posifacev1.PrintReceiptResponse{
		BytesSent: int32(len(payload)),
	}), nil
}

// ---------- Helpers ----------

func (s *Sales) draftForUpdate(tx *gorm.DB, id string) (*model.Sale, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("sale_id required"))
	}
	var sale model.Sale
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&sale).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("sale not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if sale.Status != saleStatusDraft {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("sale is %s; only DRAFT sales accept mutations", sale.Status))
	}
	return &sale, nil
}

func (s *Sales) loadFull(ctx context.Context, id string) (*model.Sale, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("sale_id required"))
	}
	var sale model.Sale
	err := s.db.WithContext(ctx).Preload("Items").Where("id = ?", id).First(&sale).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("sale not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &sale, nil
}

func recomputeSaleTotals(tx *gorm.DB, saleID string) error {
	var subtotal int64
	if err := tx.Model(&model.SaleItem{}).
		Where("sale_id = ?", saleID).
		Select("COALESCE(SUM(line_total), 0)").
		Scan(&subtotal).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	var current model.Sale
	if err := tx.Where("id = ?", saleID).First(&current).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	total := subtotal - current.CartDiscount
	if total < 0 {
		total = 0
	}
	return tx.Model(&current).Updates(map[string]any{
		"subtotal": subtotal,
		"total":    total,
	}).Error
}

func computeLineTotal(qty int32, unitPrice, lineDiscount int64) int64 {
	gross := int64(qty) * unitPrice
	net := gross - lineDiscount
	if net < 0 {
		return 0
	}
	return net
}

func assignSaleNo(tx *gorm.DB, now time.Time) (string, error) {
	year := now.Year()
	// Upsert + atomic increment.
	var counter model.SaleNoCounter
	err := tx.Where("year = ?", year).First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		counter = model.SaleNoCounter{Year: year, LastSeq: 0}
		if err := tx.Create(&counter).Error; err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}

	// Atomic increment using SQL expression to keep concurrent CompleteSale
	// calls correct under row-lock.
	if err := tx.Model(&model.SaleNoCounter{}).
		Where("year = ?", year).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Update("last_seq", gorm.Expr("last_seq + 1")).Error; err != nil {
		return "", err
	}
	if err := tx.Where("year = ?", year).First(&counter).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("INV-%d-%04d", year, counter.LastSeq), nil
}

// resolveSellUnit returns the product's selling unit for the given unit id, or
// its base unit when unitID is empty. Errors if the unit isn't sellable/active.
func resolveSellUnit(tx *gorm.DB, productID, unitID string) (*model.ProductUnit, error) {
	var u model.ProductUnit
	q := tx.Where("product_id = ? AND active", productID)
	if unitID != "" {
		q = q.Where("id = ? AND sellable", unitID)
	} else {
		q = q.Where("is_base")
	}
	if err := q.First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("selling unit not found or not sellable"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if u.Factor < 1 {
		u.Factor = 1
	}
	return &u, nil
}

// asConnectErr passes through connect errors and wraps others as Internal.
func asConnectErr(err error) error {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return err
	}
	return connect.NewError(connect.CodeInternal, err)
}

// ---------- Proto mapping ----------

func saleToProto(s *model.Sale) *posifacev1.Sale {
	out := &posifacev1.Sale{
		Id:            s.ID,
		CashierUserId: s.CashierUserID,
		Subtotal:      s.Subtotal,
		CartDiscount:  s.CartDiscount,
		Total:         s.Total,
		PaidAmount:    s.PaidAmount,
		Status:        saleStatusToProto(s.Status),
		CreatedAt:     s.CreatedAt.Unix(),
		PaymentSource: paymentSourceFromString(deref(s.PaymentSource)),
	}
	if s.SaleNo != nil {
		out.SaleNo = *s.SaleNo
	}
	if s.CustomerID != nil {
		out.CustomerId = *s.CustomerID
	}
	if s.BranchID != nil {
		out.BranchId = *s.BranchID
	}
	if s.WarehouseID != nil {
		out.WarehouseId = *s.WarehouseID
	}
	if s.CompletedAt != nil {
		out.CompletedAt = s.CompletedAt.Unix()
	}
	for i := range s.Items {
		out.Items = append(out.Items, saleItemToProto(&s.Items[i]))
	}
	return out
}

func saleItemToProto(i *model.SaleItem) *posifacev1.SaleItem {
	out := &posifacev1.SaleItem{
		Id:                i.ID,
		SaleId:            i.SaleID,
		ProductId:        i.ProductID,
		Qty:               i.Qty,
		UnitPriceSnapshot: i.UnitPriceSnapshot,
		LineDiscount:      i.LineDiscount,
		LineTotal:         i.LineTotal,
		UnitName:          i.UnitName,
		UnitFactor:        i.UnitFactor,
		BaseQty:           i.BaseQty,
	}
	if i.BatchID != nil {
		out.BatchId = *i.BatchID
	}
	if i.ProductUnitID != nil {
		out.ProductUnitId = *i.ProductUnitID
	}
	return out
}

func saleStatusToString(s posifacev1.SaleStatus) string {
	switch s {
	case posifacev1.SaleStatus_SALE_STATUS_DRAFT:
		return saleStatusDraft
	case posifacev1.SaleStatus_SALE_STATUS_COMPLETED:
		return saleStatusCompleted
	case posifacev1.SaleStatus_SALE_STATUS_VOIDED:
		return saleStatusVoided
	default:
		return ""
	}
}

func saleStatusToProto(s string) posifacev1.SaleStatus {
	switch s {
	case saleStatusDraft:
		return posifacev1.SaleStatus_SALE_STATUS_DRAFT
	case saleStatusCompleted:
		return posifacev1.SaleStatus_SALE_STATUS_COMPLETED
	case saleStatusVoided:
		return posifacev1.SaleStatus_SALE_STATUS_VOIDED
	default:
		return posifacev1.SaleStatus_SALE_STATUS_UNSPECIFIED
	}
}

func paymentSourceToString(p posifacev1.PaymentSource) (string, error) {
	switch p {
	case posifacev1.PaymentSource_PAYMENT_SOURCE_CASH:
		return paymentCash, nil
	case posifacev1.PaymentSource_PAYMENT_SOURCE_NON_CASH:
		return paymentNonCash, nil
	default:
		return "", errors.New("payment_source required")
	}
}

func paymentSourceFromString(s string) posifacev1.PaymentSource {
	switch s {
	case paymentCash:
		return posifacev1.PaymentSource_PAYMENT_SOURCE_CASH
	case paymentNonCash:
		return posifacev1.PaymentSource_PAYMENT_SOURCE_NON_CASH
	default:
		return posifacev1.PaymentSource_PAYMENT_SOURCE_UNSPECIFIED
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

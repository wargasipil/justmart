package product

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/service/common"
)

// GetProductsSummary aggregates stock across EVERY product matching the same
// filters ListProducts uses — the stat row above the catalog list, not a sum of
// the page on screen (that would change meaning as the user pages).
//
// Scoping mirrors the per-product figures exactly, so the tiles and the columns
// under them agree: ready is the caller's active warehouse, on-order stays
// company-wide (purchase_order_items carries no warehouse — the documented
// carve-out; incoming POs aren't in any warehouse yet).
func (s *ProductService) GetProductsSummary(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetProductsSummaryRequest],
) (*connect.Response[inventoryifacev1.GetProductsSummaryResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	filters, err := parseProductFilters(ctx, s.db, caller,
		req.Msg.IncludeInactive, req.Msg.OnlyArchived, req.Msg.Query, req.Msg.OpnameBefore)
	if err != nil {
		return nil, err
	}
	ids, err := filters.matchingIDs(ctx, s.db)
	if err != nil {
		return nil, err
	}
	out := &inventoryifacev1.GetProductsSummaryResponse{}
	if len(ids) == 0 {
		// An empty IN () is a dialect minefield and the answer is zeros anyway.
		return connect.NewResponse(out), nil
	}

	// Ready: count + valuation in one pass, same expression GetProduct's tile
	// uses (SUM(qty) / SUM(qty * cost_price)) so a product's row and the total
	// above it can't disagree.
	var ready struct {
		Qty       int64 `gorm:"column:qty"`
		Valuation int64 `gorm:"column:valuation"`
	}
	if err := s.db.WithContext(ctx).
		Table("stock_movements sm").
		Joins("JOIN batches b ON b.id = sm.batch_id").
		Where("b.product_id IN ? AND sm.warehouse_id = ?", ids, filters.warehouseID).
		Select("COALESCE(SUM(sm.qty), 0) AS qty, COALESCE(SUM(sm.qty * b.cost_price), 0) AS valuation").
		Scan(&ready).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out.ReadyStock = ready.Qty
	out.ReadyValuation = ready.Valuation

	// On-order: per LINE, not pre-summed in SQL, because the valuation rate is
	// netUnitCost — its rounding must match what CreateReceipt stamps on the
	// batch, or "ongoing" valuation wouldn't land where "ready" valuation will.
	// Same reason enrichStock fetches lines; see the note there.
	type orderRow struct {
		OrderedQty  int64 `gorm:"column:ordered_qty"`
		ReceivedQty int64 `gorm:"column:received_qty"`
		Subtotal    int64 `gorm:"column:subtotal"`
		UnitCost    int64 `gorm:"column:unit_cost_price"`
	}
	var orderRows []orderRow
	if err := s.db.WithContext(ctx).
		Table("purchase_order_items AS poi").
		Select("poi.ordered_qty AS ordered_qty, poi.received_qty AS received_qty, "+
			"poi.subtotal AS subtotal, poi.unit_cost_price AS unit_cost_price").
		Joins("JOIN purchase_orders po ON po.id = poi.purchase_order_id").
		Where("poi.product_id IN ? AND po.status NOT IN ?", ids,
			[]string{common.POStatusVoided, common.POStatusClosed, common.POStatusReceived}).
		Scan(&orderRows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, r := range orderRows {
		outstanding := r.OrderedQty - r.ReceivedQty
		if outstanding <= 0 {
			continue
		}
		out.OnOrderStock += outstanding
		out.OnOrderValuation += outstanding * netUnitCost(r.Subtotal, r.OrderedQty, r.UnitCost)
	}
	return connect.NewResponse(out), nil
}

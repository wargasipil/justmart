package purchasing

import (
	"context"

	"connectrpc.com/connect"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// GetPurchaseOrdersSummary aggregates the restock list's stat row: how many
// orders match the active filters, how many distinct products they cover, how
// much was ordered, and what it comes to.
//
// A server-side aggregate over EVERY matching order, not a sum of the page on
// screen — otherwise the figures would change as the user pages. It shares
// applyPOFilters with ListPurchaseOrders so the row and the table below it can
// never describe different sets (same posture as ListSales / GetSalesSummary).
//
// The status tab is just another filter here: the caller sends the tab's status
// and gets that slice, so "Draft" and "Received" each summarize themselves.
func (p *PurchaseOrders) GetPurchaseOrdersSummary(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.GetPurchaseOrdersSummaryRequest],
) (*connect.Response[purchasingifacev1.GetPurchaseOrdersSummaryResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, p.db, caller)
	if err != nil {
		return nil, err
	}
	filters := poFilterArgs{
		WarehouseID:     warehouseID,
		Status:          req.Msg.Status,
		SupplierID:      req.Msg.SupplierId,
		OnlyOutstanding: req.Msg.OnlyOutstanding,
		Query:           req.Msg.Query,
		FromUnix:        req.Msg.FromUnix,
		ToUnix:          req.Msg.ToUnix,
		DateField:       req.Msg.DateField,
	}

	// Order-level figures: one row, straight off the filtered orders.
	var head struct {
		OrderCount int64 `gorm:"column:order_count"`
		Total      int64 `gorm:"column:total"`
	}
	if err := p.applyPOFilters(p.db.WithContext(ctx).Model(&model.PurchaseOrder{}), filters).
		Select("COUNT(*) AS order_count, COALESCE(SUM(ordered_total), 0) AS total").
		Scan(&head).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Line-level figures, scoped by the same filters via a sub-select on the
	// matching order ids rather than a join — a join would multiply the
	// order-level SUM by the line count.
	ids := p.applyPOFilters(
		p.db.WithContext(ctx).Table("purchase_orders").Select("id"), filters)
	var lines struct {
		ProductCount int64 `gorm:"column:product_count"`
		ItemCount    int64 `gorm:"column:item_count"`
	}
	if err := p.db.WithContext(ctx).
		Table("purchase_order_items").
		Where("purchase_order_id IN (?)", ids).
		Select("COUNT(DISTINCT product_id) AS product_count, COALESCE(SUM(ordered_qty), 0) AS item_count").
		Scan(&lines).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&purchasingifacev1.GetPurchaseOrdersSummaryResponse{
		OrderCount:   head.OrderCount,
		ProductCount: lines.ProductCount,
		ItemCount:    lines.ItemCount,
		Total:        head.Total,
	}), nil
}

package product

import (
	"context"
	"time"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *ProductService) GetProduct(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetProductRequest],
) (*connect.Response[inventoryifacev1.GetProductResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	med, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	out := productToProto(med)
	// Fill ready_stock (active warehouse) + on_order_stock so the detail page
	// shows the same figures as the list.
	if err := s.enrichStock(ctx, caller, []*inventoryifacev1.Product{out}); err != nil {
		return nil, err
	}
	// Last restock = the most recent stock arrival INTO THE ACTIVE WAREHOUSE for
	// this product (any positive movement), with that batch's supplier.
	var rr struct {
		ReceivedAt   time.Time `gorm:"column:received_at"`
		SupplierName string    `gorm:"column:supplier_name"`
	}
	if err := s.db.WithContext(ctx).
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
		out.LastRestockDate = rr.ReceivedAt.Format(common.DateLayout)
		out.LastRestockSupplier = rr.SupplierName
	}
	// Last opname = the most recent COMPLETED stocktake session that touched any
	// batch of this product in the active warehouse, with summed variance.
	var st struct {
		CompletedAt time.Time `gorm:"column:completed_at"`
		Variance    int64     `gorm:"column:variance"`
	}
	if err := s.db.WithContext(ctx).
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
		out.LastStocktakeDate = st.CompletedAt.Format(common.DateLayout)
		out.LastStocktakeVariance = st.Variance
	}
	// Total stock (on-hand in the active warehouse) + valuation at cost.
	var v struct {
		TotalStock int64 `gorm:"column:total_stock"`
		Valuation  int64 `gorm:"column:valuation"`
	}
	if err := s.db.WithContext(ctx).
		Table("stock_movements sm").
		Joins("JOIN batches b ON b.id = sm.batch_id").
		Where("b.product_id = ? AND sm.warehouse_id = ?", med.ID, warehouseID).
		Select("COALESCE(SUM(sm.qty), 0) AS total_stock, COALESCE(SUM(sm.qty * b.cost_price), 0) AS valuation").
		Scan(&v).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out.TotalStock = v.TotalStock
	out.StockValuation = v.Valuation
	// Reference cost = the latest purchase cost (most recent batch's cost_price).
	var refCost *int64
	if err := s.db.WithContext(ctx).
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
	if err := s.attachUnits(ctx, []*inventoryifacev1.Product{out}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.GetProductResponse{Product: out}), nil
}

package supplier

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ListSupplierRestocks returns the last restock of each product from this
// supplier in the caller's active warehouse (from product_last_restocks),
// newest-arrival first, paginated. Backs the supplier-detail page.
func (s *SupplierService) ListSupplierRestocks(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListSupplierRestocksRequest],
) (*connect.Response[inventoryifacev1.ListSupplierRestocksResponse], error) {
	if req.Msg.SupplierId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("supplier_id required"))
	}
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)

	scope := func() *gorm.DB {
		q := s.db.WithContext(ctx).Model(&model.ProductLastRestock{}).
			Where("supplier_id = ? AND warehouse_id = ?", req.Msg.SupplierId, warehouseID)
		// Optional product name/sku search. product_last_restocks.product_id
		// FK-references products.id, so an id-IN subquery filters both the count
		// and the page consistently without a join (mirrors ListBatches).
		if query := strings.TrimSpace(req.Msg.Query); query != "" {
			pattern := "%" + query + "%"
			like := common.LikeOp(s.db)
			sub := s.db.Model(&model.Product{}).Select("id").
				Where("name "+like+" ? OR sku "+like+" ?", pattern, pattern)
			q = q.Where("product_id IN (?)", sub)
		}
		return q
	}

	var total int64
	if err := scope().Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var rows []model.ProductLastRestock
	if err := scope().
		Order("last_arrived_at DESC, updated_at DESC").
		Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*inventoryifacev1.SupplierProductRestock, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		out = append(out, &inventoryifacev1.SupplierProductRestock{
			ProductId:         r.ProductID,
			LastPrice:         r.LastPrice,
			LastQty:           r.LastQty,
			LastDiscountType:  r.LastDiscountType,
			LastDiscountValue: r.LastDiscountValue,
			LastCreatedAt:     unixOrZero(r.LastCreatedAt),
			LastArrivedAt:     unixOrZero(r.LastArrivedAt),
		})
	}
	return connect.NewResponse(&inventoryifacev1.ListSupplierRestocksResponse{
		Restocks: out,
		Total:    int32(total),
	}), nil
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

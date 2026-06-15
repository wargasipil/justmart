package product

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ListProductRestockLogs returns the append-only restock history for a product
// in the caller's active warehouse, newest-arrival first, paginated. Drives the
// product-detail "restock price history" tab.
func (s *ProductService) ListProductRestockLogs(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductRestockLogsRequest],
) (*connect.Response[inventoryifacev1.ListProductRestockLogsResponse], error) {
	if req.Msg.ProductId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_id required"))
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
		return s.db.WithContext(ctx).Model(&model.ProductRestockLog{}).
			Where("product_id = ? AND warehouse_id = ?", req.Msg.ProductId, warehouseID)
	}

	var total int64
	if err := scope().Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var rows []model.ProductRestockLog
	if err := scope().
		Order("restock_arrived_at DESC, created_at DESC").
		Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*inventoryifacev1.ProductRestockLog, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		out = append(out, &inventoryifacev1.ProductRestockLog{
			Id:               r.ID,
			SupplierId:       r.SupplierID,
			Price:            r.Price,
			Qty:              r.Qty,
			DiscountType:     r.DiscountType,
			DiscountValue:    r.DiscountValue,
			RestockCreatedAt: unixOrZero(r.RestockCreatedAt),
			RestockArrivedAt: unixOrZero(r.RestockArrivedAt),
		})
	}
	return connect.NewResponse(&inventoryifacev1.ListProductRestockLogsResponse{
		Logs:  out,
		Total: int32(total),
	}), nil
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

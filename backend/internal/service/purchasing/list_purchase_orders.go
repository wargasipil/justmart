package purchasing

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (p *PurchaseOrders) ListPurchaseOrders(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.ListPurchaseOrdersRequest],
) (*connect.Response[purchasingifacev1.ListPurchaseOrdersResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, p.db, caller)
	if err != nil {
		return nil, err
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	// Shared with GetPurchaseOrdersSummary — see applyPOFilters in helpers.go.
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
	applyFilters := func(q *gorm.DB) *gorm.DB { return p.applyPOFilters(q, filters) }

	var total int64
	if err := applyFilters(p.db.WithContext(ctx).Model(&model.PurchaseOrder{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var rows []model.PurchaseOrder
	if err := applyFilters(p.db.WithContext(ctx).Preload("Items")).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*purchasingifacev1.PurchaseOrder, 0, len(rows))
	for i := range rows {
		out = append(out, poToProto(&rows[i]))
	}
	if err := p.enrichList(ctx, out); err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.ListPurchaseOrdersResponse{
		Orders: out,
		Total:  int32(total),
	}), nil
}

package purchasing

import (
	"context"
	"strings"
	"time"

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
	applyFilters := func(q *gorm.DB) *gorm.DB {
		q = q.Where("warehouse_id = ?", warehouseID)
		if statusStr := poStatusToString(req.Msg.Status); statusStr != "" {
			q = q.Where("status = ?", statusStr)
		}
		if req.Msg.SupplierId != "" {
			q = q.Where("supplier_id = ?", req.Msg.SupplierId)
		}
		if req.Msg.OnlyOutstanding {
			q = q.Where("status NOT IN ?", []string{poStatusVoided, poStatusDraft}).
				Where("ordered_total > paid_amount")
		}
		if query := strings.TrimSpace(req.Msg.Query); query != "" {
			pattern := "%" + query + "%"
			sub := p.db.Table("purchase_orders AS po").
				Select("po.id").
				Joins("JOIN suppliers s ON s.id = po.supplier_id").
				Joins("LEFT JOIN purchase_order_items poi ON poi.purchase_order_id = po.id").
				Joins("LEFT JOIN products m ON m.id = poi.product_id").
				Where("po.po_no "+common.LikeOp(p.db)+" ? OR s.name "+common.LikeOp(p.db)+" ? OR s.code "+common.LikeOp(p.db)+" ? OR m.name "+common.LikeOp(p.db)+" ?",
					pattern, pattern, pattern, pattern)
			q = q.Where("id IN (?)", sub)
		}
		if req.Msg.FromUnix > 0 || req.Msg.ToUnix > 0 {
			if req.Msg.DateField == "received" {
				sub := p.db.Table("purchase_receipts").Select("purchase_order_id")
				if req.Msg.FromUnix > 0 {
					sub = sub.Where("received_at >= ?", time.Unix(req.Msg.FromUnix, 0))
				}
				if req.Msg.ToUnix > 0 {
					sub = sub.Where("received_at < ?", time.Unix(req.Msg.ToUnix, 0))
				}
				q = q.Where("id IN (?)", sub)
			} else {
				if req.Msg.FromUnix > 0 {
					q = q.Where("created_at >= ?", time.Unix(req.Msg.FromUnix, 0))
				}
				if req.Msg.ToUnix > 0 {
					q = q.Where("created_at < ?", time.Unix(req.Msg.ToUnix, 0))
				}
			}
		}
		return q
	}

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

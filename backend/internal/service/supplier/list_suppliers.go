package supplier

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SupplierService) ListSuppliers(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListSuppliersRequest],
) (*connect.Response[inventoryifacev1.ListSuppliersResponse], error) {
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	query := strings.TrimSpace(req.Msg.Query)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		if !req.Msg.IncludeInactive {
			q = q.Where("active = ?", true)
		}
		if query != "" {
			pattern := "%" + query + "%"
			q = q.Where("name "+common.LikeOp(q)+" ? OR code "+common.LikeOp(q)+" ?", pattern, pattern)
		}
		return q
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Supplier{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.Supplier
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Supplier{})).
		Order("name").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Supplier, 0, len(rows))
	for _, r := range rows {
		out = append(out, supplierToProto(&r))
	}
	return connect.NewResponse(&inventoryifacev1.ListSuppliersResponse{
		Suppliers: out,
		Total:     int32(total),
	}), nil
}

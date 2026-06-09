package supplier

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SupplierService) SearchSuppliers(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.SearchSuppliersRequest],
) (*connect.Response[inventoryifacev1.SearchSuppliersResponse], error) {
	query := strings.TrimSpace(req.Msg.Query)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := s.db.WithContext(ctx).Where("active = ?", true).Order("name").Limit(limit)
	if query != "" {
		pattern := "%" + query + "%"
		op := common.LikeOp(q)
		q = q.Where("name "+op+" ? OR code "+op+" ? OR contact_email "+op+" ? OR phone "+op+" ?",
			pattern, pattern, pattern, pattern)
	}
	var rows []model.Supplier
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Supplier, 0, len(rows))
	for _, r := range rows {
		out = append(out, supplierToProto(&r))
	}
	return connect.NewResponse(&inventoryifacev1.SearchSuppliersResponse{Suppliers: out}), nil
}

package supplier

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ResolveSuppliers returns minimal display refs for a set of ids. Unknown ids
// are omitted; empty input returns an empty list. No enrich, no preload.
func (s *SupplierService) ResolveSuppliers(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ResolveSuppliersRequest],
) (*connect.Response[inventoryifacev1.ResolveSuppliersResponse], error) {
	ids := common.DedupeIDs(req.Msg.Ids)
	if len(ids) == 0 {
		return connect.NewResponse(&inventoryifacev1.ResolveSuppliersResponse{}), nil
	}
	type row struct {
		ID   string `gorm:"column:id"`
		Code string `gorm:"column:code"`
		Name string `gorm:"column:name"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Model(&model.Supplier{}).
		Select("id, code, name").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.SupplierRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &inventoryifacev1.SupplierRef{Id: r.ID, Code: r.Code, Name: r.Name})
	}
	return connect.NewResponse(&inventoryifacev1.ResolveSuppliersResponse{Suppliers: out}), nil
}

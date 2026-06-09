package product

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ResolveProducts returns minimal display refs for a set of ids. Unknown ids
// are omitted; empty input returns an empty list. No enrich, no preload.
func (s *ProductService) ResolveProducts(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ResolveProductsRequest],
) (*connect.Response[inventoryifacev1.ResolveProductsResponse], error) {
	ids := common.DedupeIDs(req.Msg.Ids)
	if len(ids) == 0 {
		return connect.NewResponse(&inventoryifacev1.ResolveProductsResponse{}), nil
	}
	type row struct {
		ID   string `gorm:"column:id"`
		Name string `gorm:"column:name"`
		SKU  string `gorm:"column:sku"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Model(&model.Product{}).
		Select("id, name, sku").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.ProductRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &inventoryifacev1.ProductRef{Id: r.ID, Name: r.Name, Sku: r.SKU})
	}
	return connect.NewResponse(&inventoryifacev1.ResolveProductsResponse{Products: out}), nil
}

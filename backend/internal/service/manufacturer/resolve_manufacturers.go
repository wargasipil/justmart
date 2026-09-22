package manufacturer

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ResolveManufacturers returns minimal display refs for a set of ids. Unknown
// ids are omitted; empty input returns an empty list. No enrich, no preload.
//
// This is the ONE RPC in this service open to all four roles (see the proto):
// the product detail page shows "Pabrik" and is open to the till, and id/code/
// name is not cost data. ARCHIVED rows resolve too — a product may point at one,
// and the name is what the page needs to print.
func (s *ManufacturerService) ResolveManufacturers(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ResolveManufacturersRequest],
) (*connect.Response[inventoryifacev1.ResolveManufacturersResponse], error) {
	ids := common.DedupeIDs(req.Msg.Ids)
	if len(ids) == 0 {
		return connect.NewResponse(&inventoryifacev1.ResolveManufacturersResponse{}), nil
	}
	type row struct {
		ID   string `gorm:"column:id"`
		Code string `gorm:"column:code"`
		Name string `gorm:"column:name"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Model(&model.Manufacturer{}).
		Select("id, code, name").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.ManufacturerRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &inventoryifacev1.ManufacturerRef{Id: r.ID, Code: r.Code, Name: r.Name})
	}
	return connect.NewResponse(&inventoryifacev1.ResolveManufacturersResponse{Manufacturers: out}), nil
}

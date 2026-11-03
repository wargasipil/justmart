package manufacturer

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// UnarchiveManufacturer restores an archived row. The name index only covers
// ACTIVE rows, so another manufacturer may have taken this name meanwhile —
// that collision has to be reported as a field token, not as a raw index error.
func (s *ManufacturerService) UnarchiveManufacturer(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UnarchiveManufacturerRequest],
) (*connect.Response[inventoryifacev1.UnarchiveManufacturerResponse], error) {
	m, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	db := s.db.WithContext(ctx)
	if err := assertUnique(db, m.Code, m.Name, m.ID); err != nil {
		return nil, err
	}
	if err := db.Model(m).Update("active", true).Error; err != nil {
		return nil, common.AsConnectErr(err)
	}
	m.Active = true
	return connect.NewResponse(&inventoryifacev1.UnarchiveManufacturerResponse{
		Manufacturer: manufacturerToProto(m),
	}), nil
}

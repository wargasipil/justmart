package manufacturer

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

// ArchiveManufacturer soft-deletes (active = false). Products keep pointing at
// it: the link is a historical fact, and blanking it would lose who made the
// stock on the shelf. The row simply stops being offered in the picker.
func (s *ManufacturerService) ArchiveManufacturer(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ArchiveManufacturerRequest],
) (*connect.Response[inventoryifacev1.ArchiveManufacturerResponse], error) {
	m, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(m).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	m.Active = false
	return connect.NewResponse(&inventoryifacev1.ArchiveManufacturerResponse{
		Manufacturer: manufacturerToProto(m),
	}), nil
}

package manufacturer

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

func (s *ManufacturerService) UpdateManufacturer(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UpdateManufacturerRequest],
) (*connect.Response[inventoryifacev1.UpdateManufacturerResponse], error) {
	m, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Msg.Name)
	code := strings.ToUpper(strings.TrimSpace(req.Msg.Code))
	if name == "" || code == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "manufacturer.required")
	}
	db := s.db.WithContext(ctx)
	// Excludes self, so re-saving an unchanged row is not a collision.
	if err := assertUnique(db, code, name, m.ID); err != nil {
		return nil, err
	}
	m.Code = code
	m.Name = name
	m.Address = strings.TrimSpace(req.Msg.Address)
	m.Phone = strings.TrimSpace(req.Msg.Phone)
	m.ContactEmail = strings.TrimSpace(req.Msg.ContactEmail)
	m.Note = strings.TrimSpace(req.Msg.Note)
	if err := db.Model(m).Select(
		"code", "name", "address", "phone", "contact_email", "note",
	).Updates(m).Error; err != nil {
		return nil, common.AsConnectErr(err)
	}
	return connect.NewResponse(&inventoryifacev1.UpdateManufacturerResponse{
		Manufacturer: manufacturerToProto(m),
	}), nil
}

package manufacturer

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *ManufacturerService) CreateManufacturer(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.CreateManufacturerRequest],
) (*connect.Response[inventoryifacev1.CreateManufacturerResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	code := strings.ToUpper(strings.TrimSpace(req.Msg.Code))
	if name == "" || code == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "manufacturer.required")
	}
	db := s.db.WithContext(ctx)
	if err := assertUnique(db, code, name, ""); err != nil {
		return nil, err
	}
	m := model.Manufacturer{
		Code:         code,
		Name:         name,
		Address:      strings.TrimSpace(req.Msg.Address),
		Phone:        strings.TrimSpace(req.Msg.Phone),
		ContactEmail: strings.TrimSpace(req.Msg.ContactEmail),
		Note:         strings.TrimSpace(req.Msg.Note),
		Active:       true,
	}
	if err := db.Create(&m).Error; err != nil {
		// Backstop for the rare race that loses the pre-check. RE-CHECK rather
		// than assume: reporting every insert failure as "code_taken" is how a
		// schema fault gets misread as a duplicate (see suppliers/00052).
		if e := assertUnique(db, code, name, ""); e != nil {
			return nil, e
		}
		return nil, common.AsConnectErr(err)
	}
	return connect.NewResponse(&inventoryifacev1.CreateManufacturerResponse{
		Manufacturer: manufacturerToProto(&m),
	}), nil
}

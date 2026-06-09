package unit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"

	unitifacev1 "github.com/justmart/backend/gen/unit_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (u *UnitService) CreateUnitBase(
	ctx context.Context,
	req *connect.Request[unitifacev1.CreateUnitBaseRequest],
) (*connect.Response[unitifacev1.CreateUnitBaseResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	row := model.UnitBase{Name: name, Active: true}
	if err := u.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists,
			fmt.Errorf("base unit %q already exists or DB error: %w", name, err))
	}
	return connect.NewResponse(&unitifacev1.CreateUnitBaseResponse{
		Base: baseToProto(&row, nil),
	}), nil
}

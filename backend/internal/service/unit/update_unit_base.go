package unit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	unitifacev1 "github.com/justmart/backend/gen/unit_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (u *UnitService) UpdateUnitBase(
	ctx context.Context,
	req *connect.Request[unitifacev1.UpdateUnitBaseRequest],
) (*connect.Response[unitifacev1.UpdateUnitBaseResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	var row model.UnitBase
	if err := u.db.WithContext(ctx).First(&row, "id = ?", req.Msg.Id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("base unit not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := u.db.WithContext(ctx).Model(&row).Update("name", name).Error; err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists,
			fmt.Errorf("base unit %q already exists or DB error: %w", name, err))
	}
	row.Name = name
	return connect.NewResponse(&unitifacev1.UpdateUnitBaseResponse{
		Base: baseToProto(&row, nil),
	}), nil
}

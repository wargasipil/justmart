package unit

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	unitifacev1 "github.com/justmart/backend/gen/unit_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (u *UnitService) ArchiveUnitBase(
	ctx context.Context,
	req *connect.Request[unitifacev1.ArchiveUnitBaseRequest],
) (*connect.Response[unitifacev1.ArchiveUnitBaseResponse], error) {
	var row model.UnitBase
	if err := u.db.WithContext(ctx).First(&row, "id = ?", req.Msg.Id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("base unit not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := u.db.WithContext(ctx).Model(&row).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	row.Active = false
	return connect.NewResponse(&unitifacev1.ArchiveUnitBaseResponse{
		Base: baseToProto(&row, nil),
	}), nil
}

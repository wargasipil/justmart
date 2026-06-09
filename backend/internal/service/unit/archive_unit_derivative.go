package unit

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	unitifacev1 "github.com/justmart/backend/gen/unit_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (u *UnitService) ArchiveUnitDerivative(
	ctx context.Context,
	req *connect.Request[unitifacev1.ArchiveUnitDerivativeRequest],
) (*connect.Response[unitifacev1.ArchiveUnitDerivativeResponse], error) {
	var row model.UnitDerivative
	if err := u.db.WithContext(ctx).First(&row, "id = ?", req.Msg.Id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("derivative not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := u.db.WithContext(ctx).Model(&row).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	row.Active = false
	return connect.NewResponse(&unitifacev1.ArchiveUnitDerivativeResponse{
		Derivative: derivativeToProto(&row),
	}), nil
}

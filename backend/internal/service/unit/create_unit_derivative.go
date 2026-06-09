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

func (u *UnitService) CreateUnitDerivative(
	ctx context.Context,
	req *connect.Request[unitifacev1.CreateUnitDerivativeRequest],
) (*connect.Response[unitifacev1.CreateUnitDerivativeResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	if req.Msg.Factor <= 1 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("factor must be > 1"))
	}
	// Confirm the base exists.
	var base model.UnitBase
	if err := u.db.WithContext(ctx).First(&base, "id = ?", req.Msg.BaseUnitId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("base unit not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	row := model.UnitDerivative{
		BaseUnitID: req.Msg.BaseUnitId,
		Name:       name,
		Factor:     req.Msg.Factor,
		SortOrder:  req.Msg.SortOrder,
		Active:     true,
	}
	if err := u.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists,
			fmt.Errorf("derivative %q already exists for this base or DB error: %w", name, err))
	}
	return connect.NewResponse(&unitifacev1.CreateUnitDerivativeResponse{
		Derivative: derivativeToProto(&row),
	}), nil
}

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
	"github.com/justmart/backend/internal/service/common"
)

func (u *UnitService) UpdateUnitDerivative(
	ctx context.Context,
	req *connect.Request[unitifacev1.UpdateUnitDerivativeRequest],
) (*connect.Response[unitifacev1.UpdateUnitDerivativeResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "unit.name_required")
	}
	if req.Msg.Factor <= 1 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "unit.factor_invalid")
	}
	var row model.UnitDerivative
	if err := u.db.WithContext(ctx).First(&row, "id = ?", req.Msg.Id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("derivative not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := u.db.WithContext(ctx).Model(&row).Updates(map[string]any{
		"name":       name,
		"factor":     req.Msg.Factor,
		"sort_order": req.Msg.SortOrder,
	}).Error; err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists,
			fmt.Errorf("derivative %q already exists for this base or DB error: %w", name, err))
	}
	row.Name = name
	row.Factor = req.Msg.Factor
	row.SortOrder = req.Msg.SortOrder
	return connect.NewResponse(&unitifacev1.UpdateUnitDerivativeResponse{
		Derivative: derivativeToProto(&row),
	}), nil
}

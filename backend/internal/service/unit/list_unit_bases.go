package unit

import (
	"context"

	"connectrpc.com/connect"

	unitifacev1 "github.com/justmart/backend/gen/unit_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (u *UnitService) ListUnitBases(
	ctx context.Context,
	req *connect.Request[unitifacev1.ListUnitBasesRequest],
) (*connect.Response[unitifacev1.ListUnitBasesResponse], error) {
	var bases []model.UnitBase
	q := u.db.WithContext(ctx).Order("name")
	if !req.Msg.IncludeInactive {
		q = q.Where("active")
	}
	if err := q.Find(&bases).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(bases) == 0 {
		return connect.NewResponse(&unitifacev1.ListUnitBasesResponse{}), nil
	}
	ids := make([]string, 0, len(bases))
	for _, b := range bases {
		ids = append(ids, b.ID)
	}
	var derivs []model.UnitDerivative
	dq := u.db.WithContext(ctx).Where("base_unit_id IN ?", ids).Order("sort_order, name")
	if !req.Msg.IncludeInactive {
		dq = dq.Where("active")
	}
	if err := dq.Find(&derivs).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	byBase := make(map[string][]*unitifacev1.UnitDerivative, len(bases))
	for i := range derivs {
		d := &derivs[i]
		byBase[d.BaseUnitID] = append(byBase[d.BaseUnitID], derivativeToProto(d))
	}
	out := make([]*unitifacev1.UnitBase, 0, len(bases))
	for i := range bases {
		b := &bases[i]
		out = append(out, &unitifacev1.UnitBase{
			Id:          b.ID,
			Name:        b.Name,
			Active:      b.Active,
			CreatedAt:   b.CreatedAt.Unix(),
			Derivatives: byBase[b.ID],
		})
	}
	return connect.NewResponse(&unitifacev1.ListUnitBasesResponse{Bases: out}), nil
}

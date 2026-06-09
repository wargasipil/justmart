package unit

import (
	unitifacev1 "github.com/justmart/backend/gen/unit_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func baseToProto(b *model.UnitBase, derivs []*unitifacev1.UnitDerivative) *unitifacev1.UnitBase {
	return &unitifacev1.UnitBase{
		Id:          b.ID,
		Name:        b.Name,
		Active:      b.Active,
		CreatedAt:   b.CreatedAt.Unix(),
		Derivatives: derivs,
	}
}

func derivativeToProto(d *model.UnitDerivative) *unitifacev1.UnitDerivative {
	return &unitifacev1.UnitDerivative{
		Id:         d.ID,
		BaseUnitId: d.BaseUnitID,
		Name:       d.Name,
		Factor:     d.Factor,
		SortOrder:  d.SortOrder,
		Active:     d.Active,
	}
}

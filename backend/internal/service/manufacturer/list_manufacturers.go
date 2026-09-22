package manufacturer

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *ManufacturerService) ListManufacturers(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListManufacturersRequest],
) (*connect.Response[inventoryifacev1.ListManufacturersResponse], error) {
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	applyFilters := manufacturerFilters(req.Msg.IncludeInactive, req.Msg.Query)
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Manufacturer{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.Manufacturer
	// `name, id` — the id tiebreaker keeps paging stable when two rows share a
	// name (possible once one of them is archived).
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Manufacturer{})).
		Order("name, id").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Manufacturer, 0, len(rows))
	for i := range rows {
		out = append(out, manufacturerToProto(&rows[i]))
	}
	return connect.NewResponse(&inventoryifacev1.ListManufacturersResponse{
		Manufacturers: out,
		Total:         int32(total),
	}), nil
}

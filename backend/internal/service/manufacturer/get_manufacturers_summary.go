package manufacturer

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// GetManufacturersSummary counts every manufacturer matching the same filters
// ListManufacturers uses — the stat tile above the list, not a count of the page
// on screen (that would change meaning as the user pages).
//
// Both handlers go through manufacturerFilters, so the tile and the table under
// it can never describe different sets.
func (s *ManufacturerService) GetManufacturersSummary(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetManufacturersSummaryRequest],
) (*connect.Response[inventoryifacev1.GetManufacturersSummaryResponse], error) {
	applyFilters := manufacturerFilters(req.Msg.IncludeInactive, req.Msg.Query)
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Manufacturer{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&inventoryifacev1.GetManufacturersSummaryResponse{
		TotalManufacturers: total,
	}), nil
}

package supplier

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// GetSuppliersSummary counts every supplier matching the same filters
// ListSuppliers uses — the stat row above the list, not a count of the page on
// screen (that would change meaning as the user pages).
//
// Both handlers go through supplierFilters, so the tile and the table under it
// can never describe different sets.
func (s *SupplierService) GetSuppliersSummary(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetSuppliersSummaryRequest],
) (*connect.Response[inventoryifacev1.GetSuppliersSummaryResponse], error) {
	applyFilters := supplierFilters(req.Msg.IncludeInactive, req.Msg.Query)
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Supplier{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&inventoryifacev1.GetSuppliersSummaryResponse{
		TotalSuppliers: total,
	}), nil
}

package table

import (
	"context"

	"connectrpc.com/connect"

	tableifacev1 "github.com/justmart/backend/gen/table_iface/v1"
)

// UnarchiveTable returns a retired table to the floor.
//
// The code uniqueness check runs again here, and it has to: the partial unique
// index only covers ACTIVE rows, so while this table sat archived its code may
// well have been reissued to another table. Restoring blindly would either
// violate the index or put two "T1"s on the same floor plan.
func (s *TableService) UnarchiveTable(
	ctx context.Context,
	req *connect.Request[tableifacev1.UnarchiveTableRequest],
) (*connect.Response[tableifacev1.UnarchiveTableResponse], error) {
	t, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if !t.Active {
		if err := assertCodeFree(s.db.WithContext(ctx), t.WarehouseID, t.Code, t.ID); err != nil {
			return nil, err
		}
		t.Active = true
		if err := s.db.WithContext(ctx).Model(t).
			Select("active", "updated_at").Updates(t).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&tableifacev1.UnarchiveTableResponse{Table: toProto(t)}), nil
}

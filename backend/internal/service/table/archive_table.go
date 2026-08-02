package table

import (
	"context"

	"connectrpc.com/connect"

	tableifacev1 "github.com/justmart/backend/gen/table_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// ArchiveTable retires a table from the floor (soft delete — completed sales
// still reference it).
//
// A table with an OPEN BILL cannot be archived: the bill would vanish from the
// floor plan while still being a live DRAFT holding a seated customer's order.
// Settle it or move it first.
func (s *TableService) ArchiveTable(
	ctx context.Context,
	req *connect.Request[tableifacev1.ArchiveTableRequest],
) (*connect.Response[tableifacev1.ArchiveTableResponse], error) {
	t, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	bills, err := loadOpenBills(ctx, s.db, []string{t.ID})
	if err != nil {
		return nil, err
	}
	if _, occupied := bills[t.ID]; occupied {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "table.occupied")
	}
	if t.Active {
		t.Active = false
		if err := s.db.WithContext(ctx).Model(t).
			Select("active", "updated_at").Updates(t).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&tableifacev1.ArchiveTableResponse{Table: toProto(t)}), nil
}

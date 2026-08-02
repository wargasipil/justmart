package table

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	tableifacev1 "github.com/justmart/backend/gen/table_iface/v1"
)

// UpdateTable edits a table's label, area and capacity. The outlet is immutable:
// moving a table between outlets would silently re-home whatever bill is open on
// it, and the real-world action (a table exists in one room) has no counterpart.
func (s *TableService) UpdateTable(
	ctx context.Context,
	req *connect.Request[tableifacev1.UpdateTableRequest],
) (*connect.Response[tableifacev1.UpdateTableResponse], error) {
	t, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	code, err := validateTable(req.Msg.Code, req.Msg.Seats)
	if err != nil {
		return nil, err
	}
	db := s.db.WithContext(ctx)
	if code != t.Code {
		if err := assertCodeFree(db, t.WarehouseID, code, t.ID); err != nil {
			return nil, err
		}
	}

	t.Code = code
	t.Name = strings.TrimSpace(req.Msg.Name)
	t.Area = strings.TrimSpace(req.Msg.Area)
	t.Seats = req.Msg.Seats
	if err := db.Model(t).
		Select("code", "name", "area", "seats", "updated_at").
		Updates(t).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := toProto(t)
	bills, err := loadOpenBills(ctx, s.db, []string{t.ID})
	if err != nil {
		return nil, err
	}
	applyOccupancy(out, bills)
	return connect.NewResponse(&tableifacev1.UpdateTableResponse{Table: out}), nil
}

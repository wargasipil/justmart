package table

import (
	"context"

	"connectrpc.com/connect"

	tableifacev1 "github.com/justmart/backend/gen/table_iface/v1"
)

func (s *TableService) GetTable(
	ctx context.Context,
	req *connect.Request[tableifacev1.GetTableRequest],
) (*connect.Response[tableifacev1.GetTableResponse], error) {
	t, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	out := toProto(t)
	bills, err := loadOpenBills(ctx, s.db, []string{t.ID})
	if err != nil {
		return nil, err
	}
	applyOccupancy(out, bills)
	return connect.NewResponse(&tableifacev1.GetTableResponse{Table: out}), nil
}

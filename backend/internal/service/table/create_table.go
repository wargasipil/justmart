package table

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	tableifacev1 "github.com/justmart/backend/gen/table_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// CreateTable adds a table to the CALLER'S ACTIVE WAREHOUSE. The outlet is taken
// from the warehouse header rather than the request: a manager creating tables is
// already working "in" an outlet (that is what the TopBar selector means), and
// accepting an id here would let one outlet's floor be edited from another's
// screen with no visible cue.
func (s *TableService) CreateTable(
	ctx context.Context,
	req *connect.Request[tableifacev1.CreateTableRequest],
) (*connect.Response[tableifacev1.CreateTableResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	code, err := validateTable(req.Msg.Code, req.Msg.Seats)
	if err != nil {
		return nil, err
	}
	db := s.db.WithContext(ctx)
	if err := assertCodeFree(db, warehouseID, code, ""); err != nil {
		return nil, err
	}

	t := &model.DiningTable{
		WarehouseID: warehouseID,
		Code:        code,
		Name:        strings.TrimSpace(req.Msg.Name),
		Area:        strings.TrimSpace(req.Msg.Area),
		Seats:       req.Msg.Seats,
		Active:      true,
	}
	if err := db.Create(t).Error; err != nil {
		// The partial unique index backstops the race with the pre-check.
		return nil, common.TokenError(connect.CodeAlreadyExists, "table.code_taken")
	}
	return connect.NewResponse(&tableifacev1.CreateTableResponse{Table: toProto(t)}), nil
}

package table_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	tableifacev1 "github.com/justmart/backend/gen/table_iface/v1"
)

func TestCreateTable_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newTableEnv(t)

	tbl := createTable(t, svc, ctx, "T1")
	require.NotEmpty(t, tbl.Id)
	require.Equal(t, "T1", tbl.Code)
	require.Equal(t, "Indoor", tbl.Area)
	require.Equal(t, int32(4), tbl.Seats)
	require.True(t, tbl.Active)
	// The outlet comes from the caller's active warehouse, not the request.
	require.Equal(t, defaultWarehouseID(t, db), tbl.WarehouseId)
	// A fresh table is free.
	require.Empty(t, tbl.OpenSaleId)
}

func TestCreateTable_RequiresCode(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)

	_, err := svc.CreateTable(ctx, connect.NewRequest(&tableifacev1.CreateTableRequest{Code: "   "}))
	requireToken(t, err, connect.CodeInvalidArgument, "table.code_required")
}

func TestCreateTable_RejectsNegativeSeats(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)

	_, err := svc.CreateTable(ctx, connect.NewRequest(&tableifacev1.CreateTableRequest{
		Code: "T9", Seats: -1,
	}))
	requireToken(t, err, connect.CodeInvalidArgument, "table.seats_invalid")
}

// Codes identify a table on the floor plan, so two live "T1"s in one outlet
// would make the plan ambiguous.
func TestCreateTable_RejectsDuplicateCode(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	createTable(t, svc, ctx, "T1")

	_, err := svc.CreateTable(ctx, connect.NewRequest(&tableifacev1.CreateTableRequest{Code: "T1"}))
	requireToken(t, err, connect.CodeAlreadyExists, "table.code_taken")
}

// Uniqueness only covers LIVE tables, so a retired code can be reissued when the
// floor is rearranged.
func TestCreateTable_ArchivedCodeIsReusable(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	old := createTable(t, svc, ctx, "T1")
	_, err := svc.ArchiveTable(ctx, connect.NewRequest(&tableifacev1.ArchiveTableRequest{Id: old.Id}))
	require.NoError(t, err)

	fresh := createTable(t, svc, ctx, "T1")
	require.NotEqual(t, old.Id, fresh.Id)
}

func TestUpdateTable_EditsLabelAndCapacity(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	tbl := createTable(t, svc, ctx, "T1")

	resp, err := svc.UpdateTable(ctx, connect.NewRequest(&tableifacev1.UpdateTableRequest{
		Id: tbl.Id, Code: "T1A", Name: "  Corner booth  ", Area: "  Teras  ", Seats: 6,
	}))
	require.NoError(t, err)
	require.Equal(t, "T1A", resp.Msg.Table.Code)
	require.Equal(t, "Corner booth", resp.Msg.Table.Name)
	require.Equal(t, "Teras", resp.Msg.Table.Area)
	require.Equal(t, int32(6), resp.Msg.Table.Seats)
}

func TestUpdateTable_RejectsTakenCode(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	createTable(t, svc, ctx, "T1")
	second := createTable(t, svc, ctx, "T2")

	_, err := svc.UpdateTable(ctx, connect.NewRequest(&tableifacev1.UpdateTableRequest{
		Id: second.Id, Code: "T1", Seats: 2,
	}))
	requireToken(t, err, connect.CodeAlreadyExists, "table.code_taken")
}

// Keeping its own code is not a conflict with itself.
func TestUpdateTable_KeepingOwnCodeIsAllowed(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	tbl := createTable(t, svc, ctx, "T1")

	_, err := svc.UpdateTable(ctx, connect.NewRequest(&tableifacev1.UpdateTableRequest{
		Id: tbl.Id, Code: "T1", Name: "Renamed", Seats: 4,
	}))
	require.NoError(t, err)
}

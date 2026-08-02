package table_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	tableifacev1 "github.com/justmart/backend/gen/table_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// The floor lists live tables with their current bill attached, and reports
// occupancy across the whole match set.
func TestListTables_ReportsOccupancy(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	t1 := createTable(t, svc, ctx, "T1")
	createTable(t, svc, ctx, "T2")
	createTable(t, svc, ctx, "T3")
	opened, err := svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{
		TableId: t1.Id, GuestCount: 2,
	}))
	require.NoError(t, err)

	resp, err := svc.ListTables(ctx, connect.NewRequest(&tableifacev1.ListTablesRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(3), resp.Msg.Total)
	require.Equal(t, int32(1), resp.Msg.Occupied)

	byCode := map[string]*tableifacev1.DiningTable{}
	for _, tbl := range resp.Msg.Tables {
		byCode[tbl.Code] = tbl
	}
	require.Equal(t, opened.Msg.SaleId, byCode["T1"].OpenSaleId)
	require.Equal(t, int32(2), byCode["T1"].GuestCount)
	require.NotZero(t, byCode["T1"].OpenedAt)
	require.Empty(t, byCode["T2"].OpenSaleId)
}

// Archived tables are off the floor unless explicitly asked for.
func TestListTables_HidesArchivedByDefault(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	gone := createTable(t, svc, ctx, "T1")
	createTable(t, svc, ctx, "T2")
	_, err := svc.ArchiveTable(ctx, connect.NewRequest(&tableifacev1.ArchiveTableRequest{Id: gone.Id}))
	require.NoError(t, err)

	live, err := svc.ListTables(ctx, connect.NewRequest(&tableifacev1.ListTablesRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(1), live.Msg.Total)

	all, err := svc.ListTables(ctx, connect.NewRequest(&tableifacev1.ListTablesRequest{
		IncludeInactive: true,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(2), all.Msg.Total)
}

func TestListTables_OnlyOccupiedFilter(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	t1 := createTable(t, svc, ctx, "T1")
	createTable(t, svc, ctx, "T2")
	_, err := svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{TableId: t1.Id}))
	require.NoError(t, err)

	resp, err := svc.ListTables(ctx, connect.NewRequest(&tableifacev1.ListTablesRequest{
		OnlyOccupied: true,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Tables, 1)
	require.Equal(t, "T1", resp.Msg.Tables[0].Code)
	// `total` stays the pager's denominator for the same filters; `occupied` is
	// the count the header shows.
	require.Equal(t, int32(2), resp.Msg.Total)
	require.Equal(t, int32(1), resp.Msg.Occupied)
}

func TestListTables_FiltersByArea(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	createTable(t, svc, ctx, "T1") // Area "Indoor" from the fixture
	teras, err := svc.CreateTable(ctx, connect.NewRequest(&tableifacev1.CreateTableRequest{
		Code: "P1", Area: "Teras",
	}))
	require.NoError(t, err)

	resp, err := svc.ListTables(ctx, connect.NewRequest(&tableifacev1.ListTablesRequest{Area: "Teras"}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.Equal(t, teras.Msg.Table.Id, resp.Msg.Tables[0].Id)
}

// Paging returns a window while `total` and `occupied` stay whole-set figures —
// a "12/20 occupied" header must not shrink as you page.
func TestListTables_Paginates(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	for _, code := range []string{"T1", "T2", "T3", "T4", "T5"} {
		createTable(t, svc, ctx, code)
	}

	page, err := svc.ListTables(ctx, connect.NewRequest(&tableifacev1.ListTablesRequest{
		Limit: 2, Offset: 0,
	}))
	require.NoError(t, err)
	require.Len(t, page.Msg.Tables, 2)
	require.Equal(t, int32(5), page.Msg.Total)

	next, err := svc.ListTables(ctx, connect.NewRequest(&tableifacev1.ListTablesRequest{
		Limit: 2, Offset: 2,
	}))
	require.NoError(t, err)
	require.Len(t, next.Msg.Tables, 2)
	require.NotEqual(t, page.Msg.Tables[0].Id, next.Msg.Tables[0].Id)
}

// The floor is the ACTIVE WAREHOUSE's — another outlet's tables must not appear.
func TestListTables_ScopedToActiveWarehouse(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newTableEnv(t)
	createTable(t, svc, ctx, "T1")

	other := model.Warehouse{Code: "WH2", Name: "Cabang 2", Active: true}
	require.NoError(t, db.Create(&other).Error)
	require.NoError(t, db.Create(&model.DiningTable{
		WarehouseID: other.ID, Code: "X1", Active: true,
	}).Error)

	resp, err := svc.ListTables(ctx, connect.NewRequest(&tableifacev1.ListTablesRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.Equal(t, "T1", resp.Msg.Tables[0].Code)
}

// Archiving a table with a seated customer's order would hide a live DRAFT from
// the floor plan.
func TestArchiveTable_RejectsOccupiedTable(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	tbl := createTable(t, svc, ctx, "T1")
	_, err := svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{TableId: tbl.Id}))
	require.NoError(t, err)

	_, err = svc.ArchiveTable(ctx, connect.NewRequest(&tableifacev1.ArchiveTableRequest{Id: tbl.Id}))
	requireToken(t, err, connect.CodeFailedPrecondition, "table.occupied")
}

// Archive pairs with unarchive: a retired table can come back to the floor.
func TestArchiveUnarchiveTable_RoundTrip(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	tbl := createTable(t, svc, ctx, "T1")

	archived, err := svc.ArchiveTable(ctx, connect.NewRequest(&tableifacev1.ArchiveTableRequest{Id: tbl.Id}))
	require.NoError(t, err)
	require.False(t, archived.Msg.Table.Active)

	restored, err := svc.UnarchiveTable(ctx, connect.NewRequest(&tableifacev1.UnarchiveTableRequest{Id: tbl.Id}))
	require.NoError(t, err)
	require.True(t, restored.Msg.Table.Active)

	// Back on the floor and openable again.
	_, err = svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{TableId: tbl.Id}))
	require.NoError(t, err)
}

// Uniqueness only covers ACTIVE rows, so while a table sat archived its code may
// have been reissued. Restoring blindly would put two "T1"s on one floor plan.
func TestUnarchiveTable_RejectsWhenCodeWasReissued(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	old := createTable(t, svc, ctx, "T1")
	_, err := svc.ArchiveTable(ctx, connect.NewRequest(&tableifacev1.ArchiveTableRequest{Id: old.Id}))
	require.NoError(t, err)
	createTable(t, svc, ctx, "T1") // code reused by a new table

	_, err = svc.UnarchiveTable(ctx, connect.NewRequest(&tableifacev1.UnarchiveTableRequest{Id: old.Id}))
	requireToken(t, err, connect.CodeAlreadyExists, "table.code_taken")
}

// Both are idempotent — a double tap must not error.
func TestArchiveUnarchiveTable_Idempotent(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	tbl := createTable(t, svc, ctx, "T1")

	for i := 0; i < 2; i++ {
		_, err := svc.ArchiveTable(ctx, connect.NewRequest(&tableifacev1.ArchiveTableRequest{Id: tbl.Id}))
		require.NoError(t, err)
	}
	for i := 0; i < 2; i++ {
		_, err := svc.UnarchiveTable(ctx, connect.NewRequest(&tableifacev1.UnarchiveTableRequest{Id: tbl.Id}))
		require.NoError(t, err)
	}
}

func TestGetTable_NotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)

	_, err := svc.GetTable(ctx, connect.NewRequest(&tableifacev1.GetTableRequest{
		Id: "00000000-0000-0000-0000-0000000000ff",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// Sanity: the shared order-type helper is what StartSale uses to refuse DINE_IN.
func TestOrderTypeHelpers(t *testing.T) {
	t.Parallel()
	require.True(t, common.IsCounterOrderType(""))
	require.True(t, common.IsCounterOrderType(common.OrderTypeTakeaway))
	require.True(t, common.IsCounterOrderType(common.OrderTypeDelivery))
	require.False(t, common.IsCounterOrderType(common.OrderTypeDineIn))
	require.False(t, common.IsCounterOrderType("NONSENSE"))
}

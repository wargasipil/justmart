package table_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	tableifacev1 "github.com/justmart/backend/gen/table_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// Moving a bill re-seats it: the destination becomes occupied and the origin
// frees up, both derived from where the DRAFT points.
func TestMoveOrder_ReseatsTheBill(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	from := createTable(t, svc, ctx, "T1")
	to := createTable(t, svc, ctx, "T2")
	opened, err := svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{TableId: from.Id}))
	require.NoError(t, err)

	resp, err := svc.MoveOrder(ctx, connect.NewRequest(&tableifacev1.MoveOrderRequest{
		SaleId: opened.Msg.SaleId, ToTableId: to.Id,
	}))
	require.NoError(t, err)
	require.Equal(t, opened.Msg.SaleId, resp.Msg.Table.OpenSaleId)

	origin, err := svc.GetTable(ctx, connect.NewRequest(&tableifacev1.GetTableRequest{Id: from.Id}))
	require.NoError(t, err)
	require.Empty(t, origin.Msg.Table.OpenSaleId, "the origin table must be free again")
}

// A takeaway order that decides to sit down becomes DINE_IN — the honest
// description of what just happened.
func TestMoveOrder_PromotesCounterOrderToDineIn(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newTableEnv(t)
	to := createTable(t, svc, ctx, "T2")
	whID := defaultWarehouseID(t, db)

	sale := model.Sale{
		CashierUserID: ownerID,
		Status:        common.SaleStatusDraft,
		WarehouseID:   &whID,
		OrderType:     common.OrderTypeTakeaway,
	}
	require.NoError(t, db.Create(&sale).Error)

	_, err := svc.MoveOrder(ctx, connect.NewRequest(&tableifacev1.MoveOrderRequest{
		SaleId: sale.ID, ToTableId: to.Id,
	}))
	require.NoError(t, err)

	var got model.Sale
	require.NoError(t, db.Where("id = ?", sale.ID).First(&got).Error)
	require.Equal(t, common.OrderTypeDineIn, got.OrderType)
	require.NotNil(t, got.TableID)
	require.Equal(t, to.Id, *got.TableID)
}

// Merging two bills raises its own questions (whose discounts survive? which
// fired tickets?) and is out of scope — so a move onto an occupied table is
// refused rather than silently merging.
func TestMoveOrder_RejectsOccupiedDestination(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	from := createTable(t, svc, ctx, "T1")
	to := createTable(t, svc, ctx, "T2")
	opened, err := svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{TableId: from.Id}))
	require.NoError(t, err)
	_, err = svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{TableId: to.Id}))
	require.NoError(t, err)

	_, err = svc.MoveOrder(ctx, connect.NewRequest(&tableifacev1.MoveOrderRequest{
		SaleId: opened.Msg.SaleId, ToTableId: to.Id,
	}))
	requireToken(t, err, connect.CodeFailedPrecondition, "table.occupied")
}

// Moving a bill onto the table it already occupies is a no-op, not a conflict
// with itself.
func TestMoveOrder_SameTableIsNoOp(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	tbl := createTable(t, svc, ctx, "T1")
	opened, err := svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{TableId: tbl.Id}))
	require.NoError(t, err)

	resp, err := svc.MoveOrder(ctx, connect.NewRequest(&tableifacev1.MoveOrderRequest{
		SaleId: opened.Msg.SaleId, ToTableId: tbl.Id,
	}))
	require.NoError(t, err)
	require.Equal(t, opened.Msg.SaleId, resp.Msg.Table.OpenSaleId)
}

// A settled bill is history — it can't be dragged back onto the floor.
func TestMoveOrder_RejectsSettledBill(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newTableEnv(t)
	from := createTable(t, svc, ctx, "T1")
	to := createTable(t, svc, ctx, "T2")
	opened, err := svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{TableId: from.Id}))
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.Sale{}).
		Where("id = ?", opened.Msg.SaleId).
		Update("status", common.SaleStatusCompleted).Error)

	_, err = svc.MoveOrder(ctx, connect.NewRequest(&tableifacev1.MoveOrderRequest{
		SaleId: opened.Msg.SaleId, ToTableId: to.Id,
	}))
	requireToken(t, err, connect.CodeFailedPrecondition, "table.sale_not_open")
}

// Tables live in an outlet and stock is consumed from that outlet, so a bill
// can't hop between outlets by way of a table move.
func TestMoveOrder_RejectsCrossWarehouseMove(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newTableEnv(t)
	to := createTable(t, svc, ctx, "T2")

	other := model.Warehouse{Code: "WH2", Name: "Cabang 2", Active: true}
	require.NoError(t, db.Create(&other).Error)
	sale := model.Sale{
		CashierUserID: ownerID,
		Status:        common.SaleStatusDraft,
		WarehouseID:   &other.ID,
	}
	require.NoError(t, db.Create(&sale).Error)

	_, err := svc.MoveOrder(ctx, connect.NewRequest(&tableifacev1.MoveOrderRequest{
		SaleId: sale.ID, ToTableId: to.Id,
	}))
	requireToken(t, err, connect.CodeFailedPrecondition, "table.other_warehouse")
}

func TestMoveOrder_NotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	to := createTable(t, svc, ctx, "T2")

	_, err := svc.MoveOrder(ctx, connect.NewRequest(&tableifacev1.MoveOrderRequest{
		SaleId: "00000000-0000-0000-0000-0000000000ff", ToTableId: to.Id,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

package table_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	tableifacev1 "github.com/justmart/backend/gen/table_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// Opening a free table starts a DRAFT sale bound to it — the open bill. No new
// entity, no new status.
func TestOpenTable_StartsDraftBoundToTable(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newTableEnv(t)
	tbl := createTable(t, svc, ctx, "T1")

	resp, err := svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{
		TableId: tbl.Id, GuestCount: 3,
	}))
	require.NoError(t, err)
	require.False(t, resp.Msg.Resumed)
	require.NotEmpty(t, resp.Msg.SaleId)

	var sale model.Sale
	require.NoError(t, db.Where("id = ?", resp.Msg.SaleId).First(&sale).Error)
	require.Equal(t, common.SaleStatusDraft, sale.Status)
	require.NotNil(t, sale.TableID)
	require.Equal(t, tbl.Id, *sale.TableID)
	require.Equal(t, common.OrderTypeDineIn, sale.OrderType)
	require.Equal(t, int32(3), sale.GuestCount)
	require.Equal(t, ownerID, sale.CashierUserID)
	// The bill belongs to the TABLE's outlet — that is where its stock comes from.
	require.NotNil(t, sale.WarehouseID)
	require.Equal(t, tbl.WarehouseId, *sale.WarehouseID)

	// The table now reads as occupied.
	require.Equal(t, resp.Msg.SaleId, resp.Msg.Table.OpenSaleId)
	require.Equal(t, int32(3), resp.Msg.Table.GuestCount)
	require.NotZero(t, resp.Msg.Table.OpenedAt)
}

// Tapping an occupied table RESUMES its bill rather than erroring — that is how
// a waiter adds a second round, and it is the same gesture as opening.
func TestOpenTable_ResumesExistingBill(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	tbl := createTable(t, svc, ctx, "T1")

	first, err := svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{
		TableId: tbl.Id, GuestCount: 2,
	}))
	require.NoError(t, err)

	second, err := svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{
		TableId: tbl.Id, GuestCount: 9, // ignored: the bill already exists
	}))
	require.NoError(t, err)
	require.True(t, second.Msg.Resumed)
	require.Equal(t, first.Msg.SaleId, second.Msg.SaleId)
	require.Equal(t, int32(2), second.Msg.Table.GuestCount)
}

// One open bill per table is a DATABASE guarantee (partial unique index), not a
// service-level check — two waiters tapping at once is a rush-hour normality.
func TestOpenTable_OnlyOneOpenBillPerTable(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newTableEnv(t)
	tbl := createTable(t, svc, ctx, "T1")
	_, err := svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{TableId: tbl.Id}))
	require.NoError(t, err)

	// Bypass the service and try to insert a second DRAFT for the same table.
	err = db.Create(&model.Sale{
		CashierUserID: ownerID,
		Status:        common.SaleStatusDraft,
		WarehouseID:   &tbl.WarehouseId,
		TableID:       &tbl.Id,
		OrderType:     common.OrderTypeDineIn,
	}).Error
	require.Error(t, err, "the partial unique index must reject a second open bill")
}

// A settled bill frees the table with no cleanup step: occupancy is derived from
// the existence of a DRAFT, so changing the status IS releasing the table.
func TestOpenTable_CompletedBillFreesTheTable(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newTableEnv(t)
	tbl := createTable(t, svc, ctx, "T1")
	first, err := svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{TableId: tbl.Id}))
	require.NoError(t, err)

	require.NoError(t, db.Model(&model.Sale{}).
		Where("id = ?", first.Msg.SaleId).
		Update("status", common.SaleStatusCompleted).Error)

	second, err := svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{TableId: tbl.Id}))
	require.NoError(t, err)
	require.False(t, second.Msg.Resumed)
	require.NotEqual(t, first.Msg.SaleId, second.Msg.SaleId)
}

func TestOpenTable_RejectsArchivedTable(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	tbl := createTable(t, svc, ctx, "T1")
	_, err := svc.ArchiveTable(ctx, connect.NewRequest(&tableifacev1.ArchiveTableRequest{Id: tbl.Id}))
	require.NoError(t, err)

	_, err = svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{TableId: tbl.Id}))
	requireToken(t, err, connect.CodeFailedPrecondition, "table.archived")
}

func TestOpenTable_RejectsNegativeGuestCount(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newTableEnv(t)
	tbl := createTable(t, svc, ctx, "T1")

	_, err := svc.OpenTable(ctx, connect.NewRequest(&tableifacev1.OpenTableRequest{
		TableId: tbl.Id, GuestCount: -1,
	}))
	requireToken(t, err, connect.CodeInvalidArgument, "table.guest_count_invalid")
}

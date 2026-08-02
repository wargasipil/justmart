package sale_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
	salesvc "github.com/justmart/backend/internal/service/sale"
	"github.com/justmart/backend/internal/service/servicetest"
)

// seedTable inserts a dining table in MAIN and returns its id.
func seedTable(t *testing.T, db *gorm.DB, code string) string {
	t.Helper()
	tbl := model.DiningTable{WarehouseID: mainWarehouseID, Code: code, Active: true}
	require.NoError(t, db.Create(&tbl).Error)
	return tbl.ID
}

// A counter order may declare how it leaves; the value round-trips onto the sale.
func TestStartSale_AcceptsCounterOrderTypes(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)

	for _, ot := range []string{"", common.OrderTypeTakeaway, common.OrderTypeDelivery} {
		resp, err := svc.StartSale(ctx, connect.NewRequest(&posifacev1.StartSaleRequest{OrderType: ot}))
		require.NoError(t, err)
		require.Equal(t, ot, resp.Msg.Sale.OrderType)
		require.Empty(t, resp.Msg.Sale.TableId)
	}
}

// DINE_IN is refused here even though it is a valid stored value: seating an
// order is TableService.OpenTable's job, and it is the only path that binds a
// table. A DINE_IN sale with no table would be a bill nobody can find.
func TestStartSale_RejectsDineIn(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)

	_, err := svc.StartSale(ctx, connect.NewRequest(&posifacev1.StartSaleRequest{
		OrderType: common.OrderTypeDineIn,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestStartSale_RejectsUnknownOrderType(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)

	_, err := svc.StartSale(ctx, connect.NewRequest(&posifacev1.StartSaleRequest{
		OrderType: "DRIVE_THRU",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// An open bill carries its floor label so POS and the receipt can print "T4"
// rather than a UUID.
func TestGetSale_CarriesTableCode(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	tableID := seedTable(t, db, "T4")
	sale := model.Sale{
		CashierUserID: ownerID,
		Status:        common.SaleStatusDraft,
		WarehouseID:   ptr(mainWarehouseID),
		TableID:       &tableID,
		OrderType:     common.OrderTypeDineIn,
		GuestCount:    2,
	}
	require.NoError(t, db.Create(&sale).Error)

	got, err := svc.GetSale(ctx, connect.NewRequest(&posifacev1.GetSaleRequest{Id: sale.ID}))
	require.NoError(t, err)
	require.Equal(t, tableID, got.Msg.Sale.TableId)
	require.Equal(t, "T4", got.Msg.Sale.TableCode)
	require.Equal(t, common.OrderTypeDineIn, got.Msg.Sale.OrderType)
	require.Equal(t, int32(2), got.Msg.Sale.GuestCount)
}

// A dining table is shared by the floor: whoever is nearest picks the bill up.
// Self-scoping an open bill to the waiter who seated the party would stop anyone
// else adding a round or taking it to the till.
func TestGetSale_OpenTableBillIsReadableByAnotherFloorUser(t *testing.T) {
	t.Parallel()
	svc, _, db, ownerID := newSaleSvc(t)
	tableID := seedTable(t, db, "T5")
	opener := seedCashier(t, db, "waiter-a@test.local")
	other := seedCashier(t, db, "waiter-b@test.local")
	_ = ownerID

	sale := model.Sale{
		CashierUserID: opener,
		Status:        common.SaleStatusDraft,
		WarehouseID:   ptr(mainWarehouseID),
		TableID:       &tableID,
		OrderType:     common.OrderTypeDineIn,
	}
	require.NoError(t, db.Create(&sale).Error)

	otherCtx := servicetest.CtxAs(context.Background(), "CASHIER", other)
	got, err := svc.GetSale(otherCtx, connect.NewRequest(&posifacev1.GetSaleRequest{Id: sale.ID}))
	require.NoError(t, err)
	require.Equal(t, sale.ID, got.Msg.Sale.Id)
}

// A sale with NO table stays self-scoped — the shared-bill exception is about
// tables, not a general relaxation.
func TestGetSale_CounterSaleStaysSelfScoped(t *testing.T) {
	t.Parallel()
	svc, _, db, _ := newSaleSvc(t)
	opener := seedCashier(t, db, "cashier-a@test.local")
	other := seedCashier(t, db, "cashier-b@test.local")

	sale := model.Sale{
		CashierUserID: opener,
		Status:        common.SaleStatusDraft,
		WarehouseID:   ptr(mainWarehouseID),
	}
	require.NoError(t, db.Create(&sale).Error)

	otherCtx := servicetest.CtxAs(context.Background(), "CASHIER", other)
	_, err := svc.GetSale(otherCtx, connect.NewRequest(&posifacev1.GetSaleRequest{Id: sale.ID}))
	require.Error(t, err)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

// An abandoned POS cart is garbage nobody will miss. An open bill on a table is
// a seated customer's order — a long service or an overnight tab must not be
// deleted by a background timer.
func TestSweepStaleDrafts_SkipsTableBoundBills(t *testing.T) {
	t.Parallel()
	_, _, db, ownerID := newSaleSvc(t)
	tableID := seedTable(t, db, "T6")

	loose := model.Sale{
		CashierUserID: ownerID,
		Status:        common.SaleStatusDraft,
		WarehouseID:   ptr(mainWarehouseID),
	}
	require.NoError(t, db.Create(&loose).Error)
	seated := model.Sale{
		CashierUserID: ownerID,
		Status:        common.SaleStatusDraft,
		WarehouseID:   ptr(mainWarehouseID),
		TableID:       &tableID,
		OrderType:     common.OrderTypeDineIn,
	}
	require.NoError(t, db.Create(&seated).Error)

	// Age both well past any idle threshold.
	old := time.Now().Add(-72 * time.Hour)
	require.NoError(t, db.Model(&model.Sale{}).
		Where("id IN ?", []string{loose.ID, seated.ID}).
		Update("updated_at", old).Error)

	n, err := salesvc.SweepStaleDrafts(context.Background(), db, time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(1), n, "only the loose cart is swept")

	var stillThere int64
	require.NoError(t, db.Model(&model.Sale{}).Where("id = ?", seated.ID).Count(&stillThere).Error)
	require.Equal(t, int64(1), stillThere, "the seated bill must survive the sweeper")

	var gone int64
	require.NoError(t, db.Model(&model.Sale{}).Where("id = ?", loose.ID).Count(&gone).Error)
	require.Zero(t, gone)
}

func ptr(s string) *string { return &s }

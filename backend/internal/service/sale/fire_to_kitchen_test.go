package sale_test

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
	salesvc "github.com/justmart/backend/internal/service/sale"
	"github.com/justmart/backend/internal/service/servicetest"
)

// newKitchenSvc is a sale service in connector print mode with a fake pusher, so
// the tests can inspect exactly what would reach a kitchen printer.
func newKitchenSvc(t *testing.T) (*salesvc.SaleService, context.Context, *gorm.DB, string, *fakePusher) {
	t.Helper()
	db, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, db, cfg)
	svc := salesvc.NewSaleService(db, cfg.Printer)
	fake := &fakePusher{}
	svc.SetConnector(config.Connector{Mode: "connector"}, fake)
	return svc, servicetest.OwnerCtx(context.Background(), ownerID), db, ownerID, fake
}

// A first fire sends every line and stamps them.
func TestFireToKitchen_SendsPendingLinesAndStampsThem(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID, fake := newKitchenSvc(t)
	nasgor := seedProduct(t, db, "menu-k1", "Nasi Goreng", 20000)
	seedStock(t, db, nasgor, ownerID, 50)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: nasgor, Qty: 2,
	}))
	require.NoError(t, err)

	resp, err := svc.FireToKitchen(ctx, connect.NewRequest(&posifacev1.FireToKitchenRequest{
		SaleId: saleID,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.FiredItems)
	require.Equal(t, int32(1), resp.Msg.Round)
	require.True(t, fake.called)
	require.Equal(t, int32(len(fake.payload)), resp.Msg.BytesSent)

	// The dish name reaches the kitchen; prices do NOT — a cook needs what to
	// make, and money on the ticket is noise at best.
	printed := string(fake.payload)
	require.Contains(t, printed, "Nasi Goreng")
	require.NotContains(t, printed, "Rp ")

	var lines []model.SaleItem
	require.NoError(t, db.Where("sale_id = ?", saleID).Find(&lines).Error)
	require.Len(t, lines, 1)
	require.NotNil(t, lines[0].FiredAt)
}

// The second fire carries ONLY the lines added since the first. Reprinting the
// whole order would have the kitchen cook the starters twice.
func TestFireToKitchen_IsIncrementalAcrossRounds(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID, fake := newKitchenSvc(t)
	nasgor := seedProduct(t, db, "menu-k2", "Nasi Goreng", 20000)
	sate := seedProduct(t, db, "menu-k3", "Sate Ayam", 25000)
	seedStock(t, db, nasgor, ownerID, 50)
	seedStock(t, db, sate, ownerID, 50)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: nasgor, Qty: 1,
	}))
	require.NoError(t, err)
	first, err := svc.FireToKitchen(ctx, connect.NewRequest(&posifacev1.FireToKitchenRequest{SaleId: saleID}))
	require.NoError(t, err)
	require.Equal(t, int32(1), first.Msg.FiredItems)

	// Second round.
	_, err = svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: sate, Qty: 2,
	}))
	require.NoError(t, err)
	second, err := svc.FireToKitchen(ctx, connect.NewRequest(&posifacev1.FireToKitchenRequest{SaleId: saleID}))
	require.NoError(t, err)
	require.Equal(t, int32(1), second.Msg.FiredItems, "only the NEW line is fired")
	require.Equal(t, int32(2), second.Msg.Round)

	printed := string(fake.payload)
	require.Contains(t, printed, "Sate Ayam")
	require.NotContains(t, printed, "Nasi Goreng", "an already-fired dish must not reprint")
	require.Contains(t, printed, "PESANAN TAMBAHAN #2")
}

// Firing with nothing new is a no-op success — a waiter double-tapping Fire
// should not be told off, and must not cause a reprint.
func TestFireToKitchen_NothingNewIsNoOp(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID, fake := newKitchenSvc(t)
	nasgor := seedProduct(t, db, "menu-k4", "Nasi Goreng", 20000)
	seedStock(t, db, nasgor, ownerID, 50)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: nasgor, Qty: 1,
	}))
	require.NoError(t, err)
	_, err = svc.FireToKitchen(ctx, connect.NewRequest(&posifacev1.FireToKitchenRequest{SaleId: saleID}))
	require.NoError(t, err)

	fake.called = false
	resp, err := svc.FireToKitchen(ctx, connect.NewRequest(&posifacev1.FireToKitchenRequest{SaleId: saleID}))
	require.NoError(t, err)
	require.Equal(t, int32(0), resp.Msg.FiredItems)
	require.False(t, fake.called, "a no-op fire must not reach the printer")
}

// A failed print leaves the lines UNFIRED so the next tap retries them. Stamping
// first would silently lose an order on any printer hiccup — in a kitchen, food
// that never gets cooked.
func TestFireToKitchen_PrintFailureLeavesLinesUnfired(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID, fake := newKitchenSvc(t)
	fake.err = connect.NewError(connect.CodeUnavailable, context.DeadlineExceeded)
	nasgor := seedProduct(t, db, "menu-k5", "Nasi Goreng", 20000)
	seedStock(t, db, nasgor, ownerID, 50)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: nasgor, Qty: 1,
	}))
	require.NoError(t, err)

	_, err = svc.FireToKitchen(ctx, connect.NewRequest(&posifacev1.FireToKitchenRequest{SaleId: saleID}))
	require.Error(t, err)

	var lines []model.SaleItem
	require.NoError(t, db.Where("sale_id = ?", saleID).Find(&lines).Error)
	require.Len(t, lines, 1)
	require.Nil(t, lines[0].FiredAt, "a failed print must be retryable")

	// Retry now succeeds and fires the same line.
	fake.err = nil
	resp, err := svc.FireToKitchen(ctx, connect.NewRequest(&posifacev1.FireToKitchenRequest{SaleId: saleID}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.FiredItems)
}

// The table code is what a cook reads from across the kitchen, so it must be on
// the ticket.
func TestFireToKitchen_PrintsTableCodeAndNote(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID, fake := newKitchenSvc(t)
	tableID := seedTable(t, db, "T7")
	es := seedProduct(t, db, "menu-k6", "Es Teh", 5000)
	seedStock(t, db, es, ownerID, 50)

	sale := model.Sale{
		CashierUserID: ownerID,
		Status:        common.SaleStatusDraft,
		WarehouseID:   ptr(mainWarehouseID),
		TableID:       &tableID,
		OrderType:     common.OrderTypeDineIn,
		GuestCount:    4,
	}
	require.NoError(t, db.Create(&sale).Error)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: sale.ID, ProductId: es, Qty: 2,
	}))
	require.NoError(t, err)
	_, err = svc.SetItemNote(ctx, connect.NewRequest(&posifacev1.SetItemNoteRequest{
		SaleId: sale.ID, ItemId: add.Msg.Sale.Items[0].Id, Note: "tanpa es",
	}))
	require.NoError(t, err)

	_, err = svc.FireToKitchen(ctx, connect.NewRequest(&posifacev1.FireToKitchenRequest{SaleId: sale.ID}))
	require.NoError(t, err)

	printed := string(fake.payload)
	require.Contains(t, printed, "T7")
	require.Contains(t, printed, "Es Teh")
	require.Contains(t, printed, "tanpa es")
	require.Contains(t, printed, "Tamu: 4")
}

// A takeaway order gets its own heading so the pass knows to bag it.
func TestFireToKitchen_HeadingReflectsOrderType(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID, fake := newKitchenSvc(t)
	nasgor := seedProduct(t, db, "menu-k7", "Nasi Goreng", 20000)
	seedStock(t, db, nasgor, ownerID, 50)

	start, err := svc.StartSale(ctx, connect.NewRequest(&posifacev1.StartSaleRequest{
		OrderType: common.OrderTypeTakeaway,
	}))
	require.NoError(t, err)
	saleID := start.Msg.Sale.Id
	_, err = svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: nasgor, Qty: 1,
	}))
	require.NoError(t, err)

	_, err = svc.FireToKitchen(ctx, connect.NewRequest(&posifacev1.FireToKitchenRequest{SaleId: saleID}))
	require.NoError(t, err)
	require.Contains(t, string(fake.payload), "BUNGKUS")
}

// Only an open bill can be fired — a settled order is history.
func TestFireToKitchen_RejectsSettledSale(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID, _ := newKitchenSvc(t)
	nasgor := seedProduct(t, db, "menu-k8", "Nasi Goreng", 20000)
	seedStock(t, db, nasgor, ownerID, 50)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: nasgor, Qty: 1,
	}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 20000,
	}))
	require.NoError(t, err)

	_, err = svc.FireToKitchen(ctx, connect.NewRequest(&posifacev1.FireToKitchenRequest{SaleId: saleID}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// The explicit request target wins; with none, the saved KITCHEN target is used;
// with neither, the receipt target — so a one-printer shop needs no setup.
func TestFireToKitchen_TargetResolutionOrder(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID, fake := newKitchenSvc(t)
	nasgor := seedProduct(t, db, "menu-k9", "Nasi Goreng", 20000)
	seedStock(t, db, nasgor, ownerID, 90)

	fireOnce := func(t *testing.T, deviceID string) {
		t.Helper()
		saleID := startDraft(t, svc, ctx)
		_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
			SaleId: saleID, ProductId: nasgor, Qty: 1,
		}))
		require.NoError(t, err)
		_, err = svc.FireToKitchen(ctx, connect.NewRequest(&posifacev1.FireToKitchenRequest{
			SaleId: saleID, ConnectorDeviceId: deviceID,
		}))
		require.NoError(t, err)
	}

	// 1. No kitchen target set at all → falls back to the receipt target.
	require.NoError(t, common.SetPrintTarget(ctx, db, "dev-till", "TILL-58"))
	fireOnce(t, "")
	require.Equal(t, "dev-till", fake.deviceID)

	// 2. A kitchen target overrides that fallback.
	require.NoError(t, common.SetKitchenPrintTarget(ctx, db, "dev-pass", "PASS-80"))
	fireOnce(t, "")
	require.Equal(t, "dev-pass", fake.deviceID)
	require.Equal(t, "PASS-80", fake.printerName)

	// 3. An explicit request target beats both.
	fireOnce(t, "dev-explicit")
	require.Equal(t, "dev-explicit", fake.deviceID)
}

// A note is cook-facing text and must never move any amount.
func TestSetItemNote_DoesNotAffectTotals(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID, _ := newKitchenSvc(t)
	es := seedProduct(t, db, "menu-n1", "Es Teh", 5000)
	seedStock(t, db, es, ownerID, 50)
	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: es, Qty: 2,
	}))
	require.NoError(t, err)
	before := add.Msg.Sale.Total

	resp, err := svc.SetItemNote(ctx, connect.NewRequest(&posifacev1.SetItemNoteRequest{
		SaleId: saleID, ItemId: add.Msg.Sale.Items[0].Id, Note: "  tanpa gula  ",
	}))
	require.NoError(t, err)
	require.Equal(t, before, resp.Msg.Sale.Total)
	require.Equal(t, "tanpa gula", resp.Msg.Sale.Items[0].KitchenNote)

	// "" clears it.
	cleared, err := svc.SetItemNote(ctx, connect.NewRequest(&posifacev1.SetItemNoteRequest{
		SaleId: saleID, ItemId: add.Msg.Sale.Items[0].Id, Note: "",
	}))
	require.NoError(t, err)
	require.Empty(t, cleared.Msg.Sale.Items[0].KitchenNote)
	require.Equal(t, before, cleared.Msg.Sale.Total)
}

func TestSetItemNote_RejectsOverlongNote(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID, _ := newKitchenSvc(t)
	es := seedProduct(t, db, "menu-n2", "Es Teh", 5000)
	seedStock(t, db, es, ownerID, 50)
	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: es, Qty: 1,
	}))
	require.NoError(t, err)

	_, err = svc.SetItemNote(ctx, connect.NewRequest(&posifacev1.SetItemNoteRequest{
		SaleId: saleID, ItemId: add.Msg.Sale.Items[0].Id, Note: strings.Repeat("x", 200),
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSetItemNote_RejectsForeignLine(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID, _ := newKitchenSvc(t)
	es := seedProduct(t, db, "menu-n3", "Es Teh", 5000)
	seedStock(t, db, es, ownerID, 50)
	saleA := startDraft(t, svc, ctx)
	saleB := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleA, ProductId: es, Qty: 1,
	}))
	require.NoError(t, err)

	_, err = svc.SetItemNote(ctx, connect.NewRequest(&posifacev1.SetItemNoteRequest{
		SaleId: saleB, ItemId: add.Msg.Sale.Items[0].Id, Note: "x",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

package transfer_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
	transfersvc "github.com/justmart/backend/internal/service/transfer"
)

func TestListTransfers_ScopedToActiveWarehouse(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := transfersvc.NewTransferService(gormDB)
	// OWNER with no WarehouseID -> ResolveWarehouse falls back to MAIN, so the
	// list is scoped to transfers touching MAIN (the source below).
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	from := defaultWarehouseID(t, gormDB)
	to := seedWarehouse(t, gormDB, "WH2", "Gudang Cabang")
	batchID := seedStockedBatch(t, gormDB, ownerID, from, 100)
	_, err := svc.CreateTransfer(ctx, connect.NewRequest(&warehouseifacev1.CreateTransferRequest{
		FromWarehouseId: from,
		ToWarehouseId:   to,
		Note:            "listed transfer",
		Lines:           []*warehouseifacev1.CreateTransferLineInput{{BatchId: batchID, Qty: 10}},
	}))
	require.NoError(t, err)

	resp, err := svc.ListTransfers(ctx, connect.NewRequest(&warehouseifacev1.ListTransfersRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.Len(t, resp.Msg.Transfers, 1)
	got := resp.Msg.Transfers[0]
	require.Equal(t, from, got.FromWarehouseId)
	require.Equal(t, to, got.ToWarehouseId)
	require.Equal(t, "listed transfer", got.Note)
	// List hydrates without lines (withLines=false), but warehouse names are set.
	require.Equal(t, "Gudang Cabang", got.ToWarehouseName)
}

func TestListTransfers_EmptyWhenNoneTouchWarehouse(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := transfersvc.NewTransferService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	from := defaultWarehouseID(t, gormDB)
	to := seedWarehouse(t, gormDB, "WH2", "Gudang Cabang")
	other := seedWarehouse(t, gormDB, "WH3", "Gudang Lain")
	batchID := seedStockedBatch(t, gormDB, ownerID, from, 100)
	_, err := svc.CreateTransfer(ctx, connect.NewRequest(&warehouseifacev1.CreateTransferRequest{
		FromWarehouseId: from,
		ToWarehouseId:   to,
		Lines:           []*warehouseifacev1.CreateTransferLineInput{{BatchId: batchID, Qty: 5}},
	}))
	require.NoError(t, err)

	// Explicit warehouse_id filter to a warehouse untouched by the transfer ->
	// empty page.
	resp, err := svc.ListTransfers(ctx, connect.NewRequest(&warehouseifacev1.ListTransfersRequest{
		WarehouseId: other,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(0), resp.Msg.Total)
	require.Empty(t, resp.Msg.Transfers)
}

func TestListTransfers_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := transfersvc.NewTransferService(gormDB)

	_, err := svc.ListTransfers(context.Background(), connect.NewRequest(&warehouseifacev1.ListTransfersRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

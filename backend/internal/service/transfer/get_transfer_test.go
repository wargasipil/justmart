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

func TestGetTransfer_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := transfersvc.NewTransferService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Seed a transfer via CreateTransfer (shares seed helpers with
	// create_transfer_test.go — same _test package).
	from := defaultWarehouseID(t, gormDB)
	to := seedWarehouse(t, gormDB, "WH2", "Gudang Cabang")
	batchID := seedStockedBatch(t, gormDB, ownerID, from, 100)
	created, err := svc.CreateTransfer(ctx, connect.NewRequest(&warehouseifacev1.CreateTransferRequest{
		FromWarehouseId: from,
		ToWarehouseId:   to,
		Note:            "for get",
		Lines:           []*warehouseifacev1.CreateTransferLineInput{{BatchId: batchID, Qty: 25}},
	}))
	require.NoError(t, err)
	id := created.Msg.Transfer.Id

	resp, err := svc.GetTransfer(ctx, connect.NewRequest(&warehouseifacev1.GetTransferRequest{Id: id}))
	require.NoError(t, err)
	tr := resp.Msg.Transfer
	require.NotNil(t, tr)
	require.Equal(t, id, tr.Id)
	require.Equal(t, created.Msg.Transfer.TransferNo, tr.TransferNo)
	require.Equal(t, from, tr.FromWarehouseId)
	require.Equal(t, to, tr.ToWarehouseId)
	require.Equal(t, "for get", tr.Note)
	require.Len(t, tr.Lines, 1)
	require.Equal(t, batchID, tr.Lines[0].BatchId)
	require.Equal(t, int32(25), tr.Lines[0].Qty)
}

func TestGetTransfer_NotFound(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := transfersvc.NewTransferService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.GetTransfer(ctx, connect.NewRequest(&warehouseifacev1.GetTransferRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

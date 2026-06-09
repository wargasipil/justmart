package batch_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	batchsvc "github.com/justmart/backend/internal/service/batch"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestUpdateBatch_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "UB-SKU-1", "Update Med")
	created, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "UB-LOT-OLD",
		ExpiryDate:      "2030-01-01",
		CostPrice:       1000,
		InitialQuantity: 7,
	}))
	require.NoError(t, err)
	batchID := created.Msg.Batch.Id

	resp, err := svc.UpdateBatch(ctx, connect.NewRequest(&inventoryifacev1.UpdateBatchRequest{
		Id:          batchID,
		BatchNumber: "UB-LOT-NEW",
		CostPrice:   2222,
		ExpiryDate:  "2032-12-31",
		ReceivedAt:  "2026-02-02",
	}))
	require.NoError(t, err)
	b := resp.Msg.Batch
	require.NotNil(t, b)
	require.Equal(t, batchID, b.Id)
	require.Equal(t, "UB-LOT-NEW", b.BatchNumber)
	require.Equal(t, int64(2222), b.CostPrice)
	require.Equal(t, "2032-12-31", b.ExpiryDate)
	require.Equal(t, "2026-02-02", b.ReceivedAt)
	// UpdateBatch reports the global current quantity (unchanged by the edit).
	require.Equal(t, int64(7), b.CurrentQuantity)
}

func TestUpdateBatch_NotFound(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := batchsvc.NewBatchService(gormDB)

	// load() rejects an unknown id with CodeNotFound before any auth/warehouse
	// work — UpdateBatch has no MustPrincipal gate of its own.
	_, err := svc.UpdateBatch(context.Background(), connect.NewRequest(&inventoryifacev1.UpdateBatchRequest{
		Id:          "22222222-2222-2222-2222-222222222222",
		BatchNumber: "x",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestUpdateBatch_BadExpiryDate(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "UB-SKU-2", "Update Med 2")
	created, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "UB-LOT-2",
		ExpiryDate:      "2030-01-01",
		CostPrice:       1000,
		InitialQuantity: 0,
	}))
	require.NoError(t, err)

	_, err = svc.UpdateBatch(ctx, connect.NewRequest(&inventoryifacev1.UpdateBatchRequest{
		Id:         created.Msg.Batch.Id,
		ExpiryDate: "not-a-date",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpdateBatch_MissingID(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := batchsvc.NewBatchService(gormDB)

	// load() rejects an empty id with CodeInvalidArgument.
	_, err := svc.UpdateBatch(context.Background(), connect.NewRequest(&inventoryifacev1.UpdateBatchRequest{
		Id: "",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

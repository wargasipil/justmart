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

func TestResolveBatches_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "RB-SKU-1", "Resolve Med")
	// Create a batch so there is a real id to resolve.
	created, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "RB-LOT-1",
		ExpiryDate:      "2030-01-01",
		CostPrice:       1000,
		InitialQuantity: 0,
	}))
	require.NoError(t, err)
	batchID := created.Msg.Batch.Id

	resp, err := svc.ResolveBatches(ctx, connect.NewRequest(&inventoryifacev1.ResolveBatchesRequest{
		Ids: []string{batchID, batchID}, // duped id is deduped by the handler
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Batches, 1)
	ref := resp.Msg.Batches[0]
	require.Equal(t, batchID, ref.Id)
	require.Equal(t, "RB-LOT-1", ref.BatchNumber)
	require.Equal(t, prodID, ref.ProductId)
	require.Equal(t, "Resolve Med", ref.ProductName)
}

func TestResolveBatches_EmptyInput(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := batchsvc.NewBatchService(gormDB)

	// Empty ids -> empty response, no error (handler short-circuits before any
	// auth/warehouse work).
	resp, err := svc.ResolveBatches(context.Background(), connect.NewRequest(&inventoryifacev1.ResolveBatchesRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Batches)
}

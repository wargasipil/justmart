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

func TestSearchBatches_ByBatchNumber(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "SB-SKU-1", "Search Med")
	_, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "SB-UNIQUE-LOT",
		ExpiryDate:      "2030-09-09",
		CostPrice:       1000,
		InitialQuantity: 5,
	}))
	require.NoError(t, err)

	resp, err := svc.SearchBatches(ctx, connect.NewRequest(&inventoryifacev1.SearchBatchesRequest{
		Query: "UNIQUE",
		Limit: 20,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Batches, 1)
	require.Equal(t, "SB-UNIQUE-LOT", resp.Msg.Batches[0].BatchNumber)
	require.Equal(t, int64(5), resp.Msg.Batches[0].CurrentQuantity)
}

func TestSearchBatches_ByProductName(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "SB-SKU-2", "Amoxicillin Forte")
	_, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "SB-LOT-2",
		ExpiryDate:      "2030-10-10",
		CostPrice:       1000,
		InitialQuantity: 0,
	}))
	require.NoError(t, err)

	resp, err := svc.SearchBatches(ctx, connect.NewRequest(&inventoryifacev1.SearchBatchesRequest{
		Query: "Amoxicillin",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Batches, 1)
	require.Equal(t, "SB-LOT-2", resp.Msg.Batches[0].BatchNumber)
}

func TestSearchBatches_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := batchsvc.NewBatchService(gormDB)

	_, err := svc.SearchBatches(context.Background(), connect.NewRequest(&inventoryifacev1.SearchBatchesRequest{
		Query: "anything",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

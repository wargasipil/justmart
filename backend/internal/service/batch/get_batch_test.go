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

func TestGetBatch_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "GB-SKU-1", "Get Med")
	created, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "GB-LOT-1",
		ExpiryDate:      "2031-03-01",
		CostPrice:       1500,
		InitialQuantity: 12,
	}))
	require.NoError(t, err)
	batchID := created.Msg.Batch.Id

	resp, err := svc.GetBatch(ctx, connect.NewRequest(&inventoryifacev1.GetBatchRequest{Id: batchID}))
	require.NoError(t, err)
	b := resp.Msg.Batch
	require.NotNil(t, b)
	require.Equal(t, batchID, b.Id)
	require.Equal(t, prodID, b.ProductId)
	require.Equal(t, "GB-LOT-1", b.BatchNumber)
	// Quantity reported for the caller's active (MAIN) warehouse.
	require.Equal(t, int64(12), b.CurrentQuantity)
}

func TestGetBatch_NotFound(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.GetBatch(ctx, connect.NewRequest(&inventoryifacev1.GetBatchRequest{
		Id: "11111111-1111-1111-1111-111111111111",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetBatch_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := batchsvc.NewBatchService(gormDB)

	_, err := svc.GetBatch(context.Background(), connect.NewRequest(&inventoryifacev1.GetBatchRequest{
		Id: "11111111-1111-1111-1111-111111111111",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

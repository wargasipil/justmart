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

func TestListBatches_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "LB-SKU-1", "List Med")

	// Two lots: one in stock, one with zero initial quantity.
	_, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "LB-LOT-INSTOCK",
		ExpiryDate:      "2030-01-01",
		CostPrice:       1000,
		InitialQuantity: 10,
	}))
	require.NoError(t, err)
	_, err = svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "LB-LOT-EMPTY",
		ExpiryDate:      "2031-01-01",
		CostPrice:       1000,
		InitialQuantity: 0,
	}))
	require.NoError(t, err)

	// Unfiltered: both lots for this product.
	resp, err := svc.ListBatches(ctx, connect.NewRequest(&inventoryifacev1.ListBatchesRequest{
		ProductId: prodID,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(2), resp.Msg.Total)
	require.Len(t, resp.Msg.Batches, 2)
	// Sorted by expiry ASC — the in-stock lot (2030) comes first.
	require.Equal(t, "LB-LOT-INSTOCK", resp.Msg.Batches[0].BatchNumber)
	require.Equal(t, int64(10), resp.Msg.Batches[0].CurrentQuantity)

	// only_in_stock filters out the zero-qty lot.
	respInStock, err := svc.ListBatches(ctx, connect.NewRequest(&inventoryifacev1.ListBatchesRequest{
		ProductId:   prodID,
		OnlyInStock: true,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), respInStock.Msg.Total)
	require.Len(t, respInStock.Msg.Batches, 1)
	require.Equal(t, "LB-LOT-INSTOCK", respInStock.Msg.Batches[0].BatchNumber)
}

func TestListBatches_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := batchsvc.NewBatchService(gormDB)

	_, err := svc.ListBatches(context.Background(), connect.NewRequest(&inventoryifacev1.ListBatchesRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

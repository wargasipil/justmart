package batch_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	batchsvc "github.com/justmart/backend/internal/service/batch"
	"github.com/justmart/backend/internal/service/servicetest"
)

// stampPackCost gives a lot the exact pack cost CreateReceipt stamps from a PO
// line priced per box (that path has its own tests in service/purchasing); the
// batch reads only have to carry it.
func stampPackCost(t *testing.T, db *gorm.DB, batchID string, price, factor int64) {
	t.Helper()
	require.NoError(t, db.Model(&model.Batch{}).Where("id = ?", batchID).
		Updates(map[string]any{"cost_pack_price": price, "cost_pack_factor": factor}).Error)
}

func TestBatchReads_CarryExactPackCost(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "PK-SKU-1", "Pack Med")
	created, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "PK-LOT-1",
		ExpiryDate:      "2030-06-30",
		ReceivedAt:      "2026-01-15",
		CostPrice:       2_167,
		InitialQuantity: 30,
	}))
	require.NoError(t, err)
	batchID := created.Msg.Batch.Id
	// A manual lot records no pack: readers derive from cost_price.
	require.Zero(t, created.Msg.Batch.CostPackPrice)
	require.Zero(t, created.Msg.Batch.CostPackFactor)

	stampPackCost(t, gormDB, batchID, 65_000, 30)

	got, err := svc.GetBatch(ctx, connect.NewRequest(&inventoryifacev1.GetBatchRequest{Id: batchID}))
	require.NoError(t, err)
	require.Equal(t, int64(65_000), got.Msg.Batch.CostPackPrice)
	require.Equal(t, int64(30), got.Msg.Batch.CostPackFactor)

	listed, err := svc.ListBatches(ctx, connect.NewRequest(&inventoryifacev1.ListBatchesRequest{ProductId: prodID}))
	require.NoError(t, err)
	require.Len(t, listed.Msg.Batches, 1)
	require.Equal(t, int64(65_000), listed.Msg.Batches[0].CostPackPrice)
	require.Equal(t, int64(30), listed.Msg.Batches[0].CostPackFactor)

	found, err := svc.SearchBatches(ctx, connect.NewRequest(&inventoryifacev1.SearchBatchesRequest{Query: "PK-LOT-1"}))
	require.NoError(t, err)
	require.Len(t, found.Msg.Batches, 1)
	require.Equal(t, int64(65_000), found.Msg.Batches[0].CostPackPrice)
	require.Equal(t, int64(30), found.Msg.Batches[0].CostPackFactor)
}

// UpdateBatch keeps the pack cost while cost_price is unchanged, and drops it
// when cost_price is corrected: the old pack figure would then describe a cost
// the lot no longer carries.
func TestUpdateBatch_CostChangeDropsPackCost(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "PK-SKU-2", "Pack Med 2")
	created, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "PK-LOT-2",
		ExpiryDate:      "2030-06-30",
		ReceivedAt:      "2026-01-15",
		CostPrice:       2_167,
		InitialQuantity: 30,
	}))
	require.NoError(t, err)
	batchID := created.Msg.Batch.Id
	stampPackCost(t, gormDB, batchID, 65_000, 30)

	same, err := svc.UpdateBatch(ctx, connect.NewRequest(&inventoryifacev1.UpdateBatchRequest{
		Id: batchID, BatchNumber: "PK-LOT-2b", CostPrice: 2_167,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(65_000), same.Msg.Batch.CostPackPrice)
	require.Equal(t, int64(30), same.Msg.Batch.CostPackFactor)

	changed, err := svc.UpdateBatch(ctx, connect.NewRequest(&inventoryifacev1.UpdateBatchRequest{
		Id: batchID, BatchNumber: "PK-LOT-2b", CostPrice: 2_200,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(2_200), changed.Msg.Batch.CostPrice)
	require.Zero(t, changed.Msg.Batch.CostPackPrice)
	require.Zero(t, changed.Msg.Batch.CostPackFactor)
}

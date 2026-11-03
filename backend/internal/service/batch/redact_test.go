package batch_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	batchsvc "github.com/justmart/backend/internal/service/batch"
	"github.com/justmart/backend/internal/service/servicetest"
)

// Batch reads are open to the till, cost and supplier included — the catalog is
// fully readable (common.CanSeeCost is true for every role). Asserted across
// all three reads because redactCost is wired into each of them separately, so
// a partial revert would show up here rather than at a call site.
func TestBatchReads_TillSeesCost(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ownerCtx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "RD-SKU-1", "Redact Med")
	sup := model.Supplier{Code: "RD-SUP-1", Name: "Redact Supplier", Active: true}
	require.NoError(t, gormDB.Create(&sup).Error)

	created, err := svc.CreateBatch(ownerCtx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		SupplierId:      sup.ID,
		BatchNumber:     "RD-LOT-1",
		ExpiryDate:      "2030-06-30",
		ReceivedAt:      "2026-01-15",
		CostPrice:       2500,
		InitialQuantity: 40,
	}))
	require.NoError(t, err)
	batchID := created.Msg.Batch.Id

	// Manager first, so the till assertions below compare against known-present
	// data rather than a fixture that happens to be empty.
	got, err := svc.GetBatch(ownerCtx, connect.NewRequest(&inventoryifacev1.GetBatchRequest{Id: batchID}))
	require.NoError(t, err)
	require.Equal(t, int64(2500), got.Msg.Batch.CostPrice)
	require.Equal(t, sup.ID, got.Msg.Batch.SupplierId)

	for _, role := range []string{"CASHIER", "APOTEKER"} {
		tillCtx := servicetest.CtxAs(context.Background(), role, ownerID)

		got, err := svc.GetBatch(tillCtx, connect.NewRequest(&inventoryifacev1.GetBatchRequest{Id: batchID}))
		require.NoError(t, err, role)
		require.Equal(t, int64(2500), got.Msg.Batch.CostPrice, role)
		require.Equal(t, sup.ID, got.Msg.Batch.SupplierId, role)
		// The lot's identity and stock come through as before.
		require.Equal(t, "RD-LOT-1", got.Msg.Batch.BatchNumber, role)
		require.Equal(t, "2030-06-30", got.Msg.Batch.ExpiryDate, role)
		require.Equal(t, int64(40), got.Msg.Batch.CurrentQuantity, role)

		listed, err := svc.ListBatches(tillCtx, connect.NewRequest(&inventoryifacev1.ListBatchesRequest{
			ProductId: prodID,
		}))
		require.NoError(t, err, role)
		require.Len(t, listed.Msg.Batches, 1, role)
		require.Equal(t, int64(2500), listed.Msg.Batches[0].CostPrice, role)
		require.Equal(t, sup.ID, listed.Msg.Batches[0].SupplierId, role)

		found, err := svc.SearchBatches(tillCtx, connect.NewRequest(&inventoryifacev1.SearchBatchesRequest{
			Query: "RD-LOT-1",
		}))
		require.NoError(t, err, role)
		require.Len(t, found.Msg.Batches, 1, role)
		require.Equal(t, int64(2500), found.Msg.Batches[0].CostPrice, role)
		require.Equal(t, sup.ID, found.Msg.Batches[0].SupplierId, role)
	}
}

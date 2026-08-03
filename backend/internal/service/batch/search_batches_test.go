package batch_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
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
	require.Equal(t, "Search Med", resp.Msg.Batches[0].ProductName) // enriched for human-readable pickers
}

// The picker renders <ProductImage> straight from the row, so the product's
// image version has to ride along on the same join that fills product_name —
// otherwise every row would need a second round trip. NULL (no picture) must
// arrive as 0, which is the client's "skip the image fetch entirely" value.
func TestSearchBatches_EnrichesProductImageVersion(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	withPic := seedProduct(t, gormDB, "SB-PIC-1", "Pictured Med")
	stamp := time.Unix(1750000000, 0).UTC()
	require.NoError(t, gormDB.Model(&model.Product{}).
		Where("id = ?", withPic).Update("image_updated_at", stamp).Error)
	noPic := seedProduct(t, gormDB, "SB-PIC-2", "Plain Med")

	for _, prodID := range []string{withPic, noPic} {
		_, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
			ProductId:       prodID,
			BatchNumber:     "SB-PIC-LOT-" + prodID[:4],
			ExpiryDate:      "2030-10-10",
			CostPrice:       1000,
			InitialQuantity: 3,
		}))
		require.NoError(t, err)
	}

	resp, err := svc.SearchBatches(ctx, connect.NewRequest(&inventoryifacev1.SearchBatchesRequest{
		Query: "SB-PIC-LOT",
		Limit: 20,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Batches, 2)

	byProduct := map[string]*inventoryifacev1.Batch{}
	for _, b := range resp.Msg.Batches {
		byProduct[b.ProductId] = b
	}
	require.Equal(t, stamp.Unix(), byProduct[withPic].ProductImageUpdatedAt)
	require.Zero(t, byProduct[noPic].ProductImageUpdatedAt, "no picture must surface as 0, not a zero-time epoch")
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

func TestSearchBatches_OnlyInStock(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "SB-SKU-STK", "Stock Test Med")
	for _, b := range []struct {
		lot string
		qty int64
		exp string
	}{
		{"SB-ZERO-LOT", 0, "2030-01-01"},
		{"SB-FULL-LOT", 7, "2030-02-02"},
	} {
		_, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
			ProductId:       prodID,
			BatchNumber:     b.lot,
			ExpiryDate:      b.exp,
			CostPrice:       1000,
			InitialQuantity: b.qty,
		}))
		require.NoError(t, err)
	}

	// Without the filter, both lots are returned.
	all, err := svc.SearchBatches(ctx, connect.NewRequest(&inventoryifacev1.SearchBatchesRequest{Query: "Stock Test"}))
	require.NoError(t, err)
	require.Len(t, all.Msg.Batches, 2)

	// only_in_stock drops the zero-quantity lot.
	inStock, err := svc.SearchBatches(ctx, connect.NewRequest(&inventoryifacev1.SearchBatchesRequest{
		Query:       "Stock Test",
		OnlyInStock: true,
	}))
	require.NoError(t, err)
	require.Len(t, inStock.Msg.Batches, 1)
	require.Equal(t, "SB-FULL-LOT", inStock.Msg.Batches[0].BatchNumber)
}

func TestSearchBatches_WarehouseScope(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "SB-SKU-WH", "Warehouse Med")
	_, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "SB-WH-LOT",
		ExpiryDate:      "2030-11-11",
		CostPrice:       1000,
		InitialQuantity: 5,
	}))
	require.NoError(t, err)

	// Empty warehouse_id → the caller's active (default MAIN) warehouse, where
	// CreateBatch stamped the PURCHASE movement.
	active, err := svc.SearchBatches(ctx, connect.NewRequest(&inventoryifacev1.SearchBatchesRequest{Query: "Warehouse Med"}))
	require.NoError(t, err)
	require.Len(t, active.Msg.Batches, 1)
	require.Equal(t, int64(5), active.Msg.Batches[0].CurrentQuantity)

	// A different warehouse holds no stock for this lot → qty 0 (still returned
	// because only_in_stock is false).
	const otherWarehouse = "00000000-0000-0000-0000-0000000000ff"
	other, err := svc.SearchBatches(ctx, connect.NewRequest(&inventoryifacev1.SearchBatchesRequest{
		Query:       "Warehouse Med",
		WarehouseId: otherWarehouse,
	}))
	require.NoError(t, err)
	require.Len(t, other.Msg.Batches, 1)
	require.Equal(t, int64(0), other.Msg.Batches[0].CurrentQuantity)

	// only_in_stock against the empty warehouse filters the lot out entirely.
	filtered, err := svc.SearchBatches(ctx, connect.NewRequest(&inventoryifacev1.SearchBatchesRequest{
		Query:       "Warehouse Med",
		WarehouseId: otherWarehouse,
		OnlyInStock: true,
	}))
	require.NoError(t, err)
	require.Empty(t, filtered.Msg.Batches)
}

func TestSearchBatches_AttachesUnits(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "SB-SKU-UNIT", "Unit Med")
	require.NoError(t, gormDB.Create(&model.ProductUnit{
		ProductID: prodID, Name: "tablet", Factor: 1, IsBase: true, Active: true,
	}).Error)
	require.NoError(t, gormDB.Create(&model.ProductUnit{
		ProductID: prodID, Name: "box", Factor: 10, Active: true,
	}).Error)
	_, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "SB-UNIT-LOT",
		ExpiryDate:      "2030-12-12",
		CostPrice:       1000,
		InitialQuantity: 30,
	}))
	require.NoError(t, err)

	resp, err := svc.SearchBatches(ctx, connect.NewRequest(&inventoryifacev1.SearchBatchesRequest{Query: "Unit Med"}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Batches, 1)
	units := resp.Msg.Batches[0].Units
	require.Len(t, units, 2)
	require.True(t, units[0].IsBase) // base first (is_base DESC)
	require.Equal(t, "tablet", units[0].Name)
	require.Equal(t, "box", units[1].Name)
	require.Equal(t, int64(10), units[1].Factor)
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

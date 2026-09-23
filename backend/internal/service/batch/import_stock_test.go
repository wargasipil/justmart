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

// seedProductUnit inserts a non-base product_units row (e.g. "box" ×100) so the
// import's unit-column resolution can be exercised. Local to the batch_test pkg.
func seedProductUnit(t *testing.T, gormDB *gorm.DB, productID, name string, factor int64) {
	t.Helper()
	u := model.ProductUnit{
		ProductID:   productID,
		Name:        name,
		Factor:      factor,
		SellPrice:   0,
		Sellable:    true,
		Purchasable: true,
		Active:      true,
	}
	require.NoError(t, gormDB.Create(&u).Error)
}

// batchStockQty sums all stock_movements for a batch (engine-agnostic).
func batchStockQty(t *testing.T, gormDB *gorm.DB, batchID string) int64 {
	t.Helper()
	var qty int64
	require.NoError(t, gormDB.Model(&model.StockMovement{}).
		Where("batch_id = ?", batchID).
		Select("COALESCE(SUM(qty), 0)").Scan(&qty).Error)
	return qty
}

func stockRow(sku string, qty int64) *inventoryifacev1.ImportStockRow {
	return &inventoryifacev1.ImportStockRow{Sku: sku, Quantity: qty}
}

func TestImportStock_HappyPath(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, gormDB, "IS-1", "Paracetamol")

	r := stockRow("IS-1", 40)
	r.CostPrice = 2500
	r.ExpiryDate = "2030-06-30"
	r.BatchNumber = "IS-LOT-1"

	resp, err := svc.ImportStock(ctx, connect.NewRequest(&inventoryifacev1.ImportStockRequest{
		Rows: []*inventoryifacev1.ImportStockRow{r},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Created)
	require.Equal(t, int32(0), resp.Msg.Skipped)
	require.Equal(t, int32(0), resp.Msg.Errored)
	require.Len(t, resp.Msg.Results, 1)
	require.Equal(t, inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_CREATED, resp.Msg.Results[0].Status)
	batchID := resp.Msg.Results[0].BatchId
	require.NotEmpty(t, batchID)

	// One PURCHASE movement of 40 landed for the new batch.
	require.Equal(t, int64(40), batchStockQty(t, gormDB, batchID))
	var b model.Batch
	require.NoError(t, gormDB.First(&b, "id = ?", batchID).Error)
	require.Equal(t, "IS-LOT-1", b.BatchNumber)
	require.Equal(t, int64(2500), b.CostPrice)
}

func TestImportStock_UnitColumnMultipliesByFactor(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "IS-UNIT", "Amoxicillin")
	seedProductUnit(t, gormDB, prodID, "box", 100)

	r := stockRow("IS-UNIT", 5)
	r.Unit = "box" // 5 box × 100 = 500 base units

	resp, err := svc.ImportStock(ctx, connect.NewRequest(&inventoryifacev1.ImportStockRequest{
		Rows: []*inventoryifacev1.ImportStockRow{r},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Created)
	require.Equal(t, int64(500), batchStockQty(t, gormDB, resp.Msg.Results[0].BatchId))
}

func TestImportStock_BlankExpiryUsesFarFuture(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, gormDB, "IS-NOEXP", "Salt")

	resp, err := svc.ImportStock(ctx, connect.NewRequest(&inventoryifacev1.ImportStockRequest{
		Rows: []*inventoryifacev1.ImportStockRow{stockRow("IS-NOEXP", 10)}, // no expiry
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Created)

	var b model.Batch
	require.NoError(t, gormDB.First(&b, "id = ?", resp.Msg.Results[0].BatchId).Error)
	require.Equal(t, 2099, b.ExpiryDate.Year()) // far-future sentinel
}

func TestImportStock_SkipsExistingBatchNumber(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, gormDB, "IS-RERUN", "Vitamin")

	row := func() *inventoryifacev1.ImportStockRow {
		r := stockRow("IS-RERUN", 20)
		r.BatchNumber = "RERUN-LOT"
		return r
	}

	// First import creates it.
	first, err := svc.ImportStock(ctx, connect.NewRequest(&inventoryifacev1.ImportStockRequest{
		Rows: []*inventoryifacev1.ImportStockRow{row()},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), first.Msg.Created)

	// Re-import the same (product, batch_number) -> SKIPPED, no double-count.
	second, err := svc.ImportStock(ctx, connect.NewRequest(&inventoryifacev1.ImportStockRequest{
		Rows: []*inventoryifacev1.ImportStockRow{row()},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(0), second.Msg.Created)
	require.Equal(t, int32(1), second.Msg.Skipped)
	require.Equal(t, inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_SKIPPED_EXISTS, second.Msg.Results[0].Status)

	// Exactly one batch exists for this lot.
	var count int64
	require.NoError(t, gormDB.Model(&model.Batch{}).Where("batch_number = ?", "RERUN-LOT").Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestImportStock_SkuNotFound(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	resp, err := svc.ImportStock(ctx, connect.NewRequest(&inventoryifacev1.ImportStockRequest{
		Rows: []*inventoryifacev1.ImportStockRow{stockRow("NOPE", 5)},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(0), resp.Msg.Created)
	require.Equal(t, int32(1), resp.Msg.Errored)
	require.Equal(t, inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_ERROR, resp.Msg.Results[0].Status)
	require.NotEmpty(t, resp.Msg.Results[0].Message)
}

func TestImportStock_UnknownUnit(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, gormDB, "IS-NOUNIT", "Syrup")

	r := stockRow("IS-NOUNIT", 3)
	r.Unit = "crate" // no such unit for this product

	resp, err := svc.ImportStock(ctx, connect.NewRequest(&inventoryifacev1.ImportStockRequest{
		Rows: []*inventoryifacev1.ImportStockRow{r},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Errored)
	require.Equal(t, inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_ERROR, resp.Msg.Results[0].Status)
	require.NotEmpty(t, resp.Msg.Results[0].Message)
}

func TestImportStock_QuantityMustBePositive(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, gormDB, "IS-ZERO", "Zero")

	resp, err := svc.ImportStock(ctx, connect.NewRequest(&inventoryifacev1.ImportStockRequest{
		Rows: []*inventoryifacev1.ImportStockRow{stockRow("IS-ZERO", 0)},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Errored)
	require.Equal(t, inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_ERROR, resp.Msg.Results[0].Status)
}

func TestImportStock_PartialSuccess(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, gormDB, "IS-OK", "Good")

	resp, err := svc.ImportStock(ctx, connect.NewRequest(&inventoryifacev1.ImportStockRequest{
		Rows: []*inventoryifacev1.ImportStockRow{
			stockRow("IS-OK", 12),  // valid
			stockRow("IS-MISS", 5), // SKU not found
			stockRow("IS-OK", -3),  // bad qty
		},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Created)
	require.Equal(t, int32(2), resp.Msg.Errored)
	require.Equal(t, inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_CREATED, resp.Msg.Results[0].Status)
	require.Equal(t, inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_ERROR, resp.Msg.Results[1].Status)
	require.Equal(t, inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_ERROR, resp.Msg.Results[2].Status)

	// The good row persisted its stock despite the bad rows.
	require.Equal(t, int64(12), batchStockQty(t, gormDB, resp.Msg.Results[0].BatchId))
}

func TestImportStock_EmptyRejected(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.ImportStock(ctx, connect.NewRequest(&inventoryifacev1.ImportStockRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestImportStock_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := batchsvc.NewBatchService(gormDB)

	_, err := svc.ImportStock(context.Background(), connect.NewRequest(&inventoryifacev1.ImportStockRequest{
		Rows: []*inventoryifacev1.ImportStockRow{stockRow("X", 1)},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

// Opening stock is the second hand-entered path that mints lots, so it can
// record the maker too -- by CODE, since a CSV is authored by a person and
// cannot carry UUIDs (the same reason the product is keyed by SKU).
func TestImportStock_RecordsTheMakerByCode(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, gormDB, "IS-MFR-1", "Paracetamol 500")
	kalbe := model.Manufacturer{Code: "IS-MFR-CODE-1", Name: "Kalbe Farma"}
	require.NoError(t, gormDB.Create(&kalbe).Error)

	r := stockRow("IS-MFR-1", 10)
	r.ManufacturerCode = kalbe.Code

	resp, err := svc.ImportStock(ctx, connect.NewRequest(&inventoryifacev1.ImportStockRequest{
		Rows: []*inventoryifacev1.ImportStockRow{r},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Created)

	var lot model.Batch
	require.NoError(t, gormDB.Where("id = ?", resp.Msg.Results[0].BatchId).First(&lot).Error)
	require.NotNil(t, lot.ManufacturerID)
	require.Equal(t, kalbe.ID, *lot.ManufacturerID, "the code resolved to the right pabrik")
}

// Blank stays blank: a row that names no pabrik yields a lot that names none,
// which is what not-recorded has to look like.
func TestImportStock_MakerOptional(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, gormDB, "IS-MFR-2", "Amoxsan")
	resp, err := svc.ImportStock(ctx, connect.NewRequest(&inventoryifacev1.ImportStockRequest{
		Rows: []*inventoryifacev1.ImportStockRow{stockRow("IS-MFR-2", 4)},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Created)

	var lot model.Batch
	require.NoError(t, gormDB.Where("id = ?", resp.Msg.Results[0].BatchId).First(&lot).Error)
	require.Nil(t, lot.ManufacturerID, "empty must be NULL, never an empty-string FK")
}

// A code that names nothing fails THAT ROW only, like an unknown unit does --
// one typo in a 400-line CSV must not reject the other 399.
func TestImportStock_UnknownMakerCodeFailsOnlyThatRow(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, gormDB, "IS-MFR-3", "Promag")
	seedProduct(t, gormDB, "IS-MFR-4", "Bodrex")

	bad := stockRow("IS-MFR-3", 5)
	bad.ManufacturerCode = "NO-SUCH-CODE"

	resp, err := svc.ImportStock(ctx, connect.NewRequest(&inventoryifacev1.ImportStockRequest{
		Rows: []*inventoryifacev1.ImportStockRow{bad, stockRow("IS-MFR-4", 6)},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Errored)
	require.Equal(t, int32(1), resp.Msg.Created)
	require.Equal(t, inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_ERROR, resp.Msg.Results[0].Status)
	require.NotEmpty(t, resp.Msg.Results[0].Message)
	require.Equal(t, inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_CREATED, resp.Msg.Results[1].Status)

	// The refused row wrote nothing at all -- its whole tx rolled back.
	var lots int64
	require.NoError(t, gormDB.Model(&model.Batch{}).
		Joins("JOIN products p ON p.id = batches.product_id").
		Where("p.sku = ?", "IS-MFR-3").Count(&lots).Error)
	require.Zero(t, lots)
}

// An ARCHIVED pabrik is accepted here for the same reason CreateBatch accepts
// one: opening stock is by definition goods that arrived before today, and some
// of it was made by factories the shop has since stopped buying from.
func TestImportStock_AcceptsAnArchivedMaker(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, gormDB, "IS-MFR-5", "Tolak Angin")
	retired := model.Manufacturer{Code: "IS-MFR-CODE-5", Name: "Retired Pharma"}
	require.NoError(t, gormDB.Create(&retired).Error)
	require.NoError(t, gormDB.Model(&model.Manufacturer{}).
		Where("id = ?", retired.ID).Update("active", false).Error)

	r := stockRow("IS-MFR-5", 7)
	r.ManufacturerCode = retired.Code
	resp, err := svc.ImportStock(ctx, connect.NewRequest(&inventoryifacev1.ImportStockRequest{
		Rows: []*inventoryifacev1.ImportStockRow{r},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Created)
}

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

// Filtering lots by their MAKER is the question the whole product-manufacturer
// split exists to answer: a recall names a pabrik, and the batch is the only row
// that records one. The lots are stamped after creation here only to keep this
// test about the FILTER; both write paths have their own coverage -- the manual
// one below, and the receipt-from-a-purchase-order-line one in the purchasing
// package.
func TestListBatches_FiltersByManufacturer(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "LBM-SKU-1", "Paracetamol 500")

	kalbe := model.Manufacturer{Code: "LBM-MFR-1", Name: "Kalbe Farma"}
	require.NoError(t, gormDB.Create(&kalbe).Error)
	dexa := model.Manufacturer{Code: "LBM-MFR-2", Name: "Dexa Medica"}
	require.NoError(t, gormDB.Create(&dexa).Error)

	seedLot := func(no string, maker *string) {
		t.Helper()
		_, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
			ProductId:       prodID,
			BatchNumber:     no,
			ExpiryDate:      "2030-01-01",
			CostPrice:       1000,
			InitialQuantity: 10,
		}))
		require.NoError(t, err)
		require.NoError(t, gormDB.Model(&model.Batch{}).
			Where("batch_number = ?", no).Update("manufacturer_id", maker).Error)
	}
	seedLot("LBM-KALBE", &kalbe.ID)
	seedLot("LBM-DEXA", &dexa.ID)
	seedLot("LBM-UNKNOWN", nil) // pre-00060 stock: the fact was never recorded

	all, err := svc.ListBatches(ctx, connect.NewRequest(&inventoryifacev1.ListBatchesRequest{
		ProductId: prodID,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(3), all.Msg.Total)

	only, err := svc.ListBatches(ctx, connect.NewRequest(&inventoryifacev1.ListBatchesRequest{
		ProductId:      prodID,
		ManufacturerId: kalbe.ID,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), only.Msg.Total, "total must reflect the filter, not the page")
	require.Len(t, only.Msg.Batches, 1)
	require.Equal(t, "LBM-KALBE", only.Msg.Batches[0].BatchNumber)
	require.Equal(t, kalbe.ID, only.Msg.Batches[0].ManufacturerId)

	// A lot with no recorded maker matches NO pabrik filter -- it is unknown,
	// not "everyone's". It only ever appears in the unfiltered list above.
	other, err := svc.ListBatches(ctx, connect.NewRequest(&inventoryifacev1.ListBatchesRequest{
		ProductId:      prodID,
		ManufacturerId: dexa.ID,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), other.Msg.Total)
	require.Equal(t, "LBM-DEXA", other.Msg.Batches[0].BatchNumber)
}

// The manual entry path can record the maker, and must: this is the one flow
// where a person is holding the physical box with the factory printed on it.
// A lot minted without one answers no recall, and the answer was in front of
// them at the time.
func TestCreateBatch_RecordsTheMaker(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "CBM-SKU-1", "Paracetamol 500")
	kalbe := model.Manufacturer{Code: "CBM-MFR-1", Name: "Kalbe Farma"}
	require.NoError(t, gormDB.Create(&kalbe).Error)

	resp, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "CBM-LOT-1",
		ExpiryDate:      "2030-01-01",
		CostPrice:       1000,
		InitialQuantity: 10,
		ManufacturerId:  kalbe.ID,
	}))
	require.NoError(t, err)
	require.Equal(t, kalbe.ID, resp.Msg.Batch.ManufacturerId, "the response carries it back")

	var lot model.Batch
	require.NoError(t, gormDB.Where("id = ?", resp.Msg.Batch.Id).First(&lot).Error)
	require.NotNil(t, lot.ManufacturerID)
	require.Equal(t, kalbe.ID, *lot.ManufacturerID)
}

// Omitting it stays legal and stores NULL, not "". The column is a nullable FK,
// and a blank has to be recognisable as not-recorded.
func TestCreateBatch_MakerIsOptionalAndStoresNull(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "CBM-SKU-2", "Amoxsan")
	resp, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "CBM-LOT-2",
		ExpiryDate:      "2030-01-01",
		CostPrice:       1000,
		InitialQuantity: 5,
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Batch.ManufacturerId)

	var lot model.Batch
	require.NoError(t, gormDB.Where("id = ?", resp.Msg.Batch.Id).First(&lot).Error)
	require.Nil(t, lot.ManufacturerID, "empty must be NULL, never an empty-string FK")
}

// An ARCHIVED pabrik is ACCEPTED on a lot, and this is the deliberate asymmetry
// with a product's approved-source list (which refuses one). A list is intent --
// you cannot start sourcing from a retired factory. A lot is a record of what
// happened, and hand-entered stock may genuinely have been made by one.
func TestCreateBatch_AcceptsAnArchivedMaker(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "CBM-SKU-3", "Promag")
	retired := model.Manufacturer{Code: "CBM-MFR-3", Name: "Retired Pharma"}
	require.NoError(t, gormDB.Create(&retired).Error)
	// Seeded active then archived: GORM omits a false bool on a default:true
	// column, so a single insert would have written active = true.
	require.NoError(t, gormDB.Model(&model.Manufacturer{}).
		Where("id = ?", retired.ID).Update("active", false).Error)

	resp, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "CBM-LOT-3",
		ExpiryDate:      "2030-01-01",
		CostPrice:       1000,
		InitialQuantity: 5,
		ManufacturerId:  retired.ID,
	}))
	require.NoError(t, err, "old stock from a retired factory is a real thing")
	require.Equal(t, retired.ID, resp.Msg.Batch.ManufacturerId)
}

// An id that names nothing is a clean NotFound rather than an opaque FK error.
func TestCreateBatch_UnknownMakerIsNotFound(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "CBM-SKU-4", "Bodrex")
	_, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "CBM-LOT-4",
		ExpiryDate:      "2030-01-01",
		CostPrice:       1000,
		InitialQuantity: 5,
		ManufacturerId:  "00000000-0000-0000-0000-0000000000ff",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

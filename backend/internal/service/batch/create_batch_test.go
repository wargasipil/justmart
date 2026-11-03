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
	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/service/servicetest"
)

// seedProduct inserts a products row directly (the SQLite UUID create-callback
// fills the id) and returns its id. batches.product_id is an FK to products, so
// every batch test seeds a product first. Local to the batch_test package.
func seedProduct(t *testing.T, gormDB *gorm.DB, sku, name string) string {
	t.Helper()
	p := model.Product{
		SKU:       sku,
		Name:      name,
		Unit:      "tablet",
		UnitPrice: 5000,
		Active:    true,
	}
	require.NoError(t, gormDB.Create(&p).Error)
	require.NotEmpty(t, p.ID)
	return p.ID
}

func TestCreateBatch_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg) // real users.id for the movement FK
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "CB-SKU-1", "Create Med")

	resp, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		BatchNumber:     "CB-LOT-1",
		ExpiryDate:      "2030-06-30",
		ReceivedAt:      "2026-01-15",
		CostPrice:       2500,
		InitialQuantity: 40,
	}))
	require.NoError(t, err)
	b := resp.Msg.Batch
	require.NotNil(t, b)
	require.NotEmpty(t, b.Id) // UUID filled by the SQLite create-callback
	require.Equal(t, prodID, b.ProductId)
	require.Equal(t, "CB-LOT-1", b.BatchNumber)
	require.Equal(t, "2030-06-30", b.ExpiryDate)
	require.Equal(t, "2026-01-15", b.ReceivedAt)
	require.Equal(t, int64(2500), b.CostPrice)
	require.Equal(t, int64(40), b.CurrentQuantity) // initial PURCHASE movement

	// The initial PURCHASE movement landed in the (default MAIN) warehouse with
	// the caller stamped as the user.
	var mvCount int64
	require.NoError(t, gormDB.Model(&model.StockMovement{}).
		Where("batch_id = ? AND type = ? AND qty = ?", b.Id, "PURCHASE", 40).
		Count(&mvCount).Error)
	require.Equal(t, int64(1), mvCount)
}

func TestCreateBatch_MissingProductID(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:  "", // required
		ExpiryDate: "2030-06-30",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateBatch_BadExpiryDate(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "CB-SKU-2", "Create Med 2")

	_, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:  prodID,
		ExpiryDate: "30-06-2030", // not YYYY-MM-DD
	}))
	requireBatchToken(t, err, connect.CodeInvalidArgument, "batch.expiry_invalid")
}

// The drawer reports where the date came from; DEFAULT survives onto the lot,
// and NONE stores the no-expiry placeholder whatever date was sent.
func TestCreateBatch_RecordsExpirySource(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	prodID := seedProduct(t, gormDB, "CB-SRC", "Source Med")

	def, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:    prodID,
		ExpiryDate:   "2030-06-30",
		ExpirySource: inventoryifacev1.ExpirySource_EXPIRY_SOURCE_DEFAULT,
	}))
	require.NoError(t, err)
	require.Equal(t, inventoryifacev1.ExpirySource_EXPIRY_SOURCE_DEFAULT, def.Msg.Batch.ExpirySource)
	require.Equal(t, "2030-06-30", def.Msg.Batch.ExpiryDate)

	none, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:    prodID,
		ExpirySource: inventoryifacev1.ExpirySource_EXPIRY_SOURCE_NONE, // no date at all
	}))
	require.NoError(t, err)
	require.Equal(t, inventoryifacev1.ExpirySource_EXPIRY_SOURCE_NONE, none.Msg.Batch.ExpirySource)
	require.Equal(t, "2099-12-31", none.Msg.Batch.ExpiryDate)

	// A caller that predates the field sent a typed date: ENTERED.
	plain, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:  prodID,
		ExpiryDate: "2031-01-31",
	}))
	require.NoError(t, err)
	require.Equal(t, inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED, plain.Msg.Batch.ExpirySource)
}

// In pharmacy mode a prescription medicine cannot be recorded as never
// expiring; the same product in retail mode can.
func TestCreateBatch_PharmacyRxMustExpire(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	prodID := seedProduct(t, gormDB, "CB-RX", "Rx Med")
	require.NoError(t, gormDB.Model(&model.Product{}).Where("id = ?", prodID).
		Update("prescription_required", true).Error)
	noExpiry := &inventoryifacev1.CreateBatchRequest{
		ProductId:    prodID,
		ExpirySource: inventoryifacev1.ExpirySource_EXPIRY_SOURCE_NONE,
	}

	_, err := svc.CreateBatch(ctx, connect.NewRequest(noExpiry))
	require.NoError(t, err, "retail mode: the pharmacy rule does not apply")

	require.NoError(t, common.SetBussinessType(ctx, gormDB, common.BussinessTypePharmacyShop))
	_, err = svc.CreateBatch(ctx, connect.NewRequest(noExpiry))
	requireBatchToken(t, err, connect.CodeFailedPrecondition, "batch.expiry_required")
}

func requireBatchToken(t *testing.T, err error, code connect.Code, token string) {
	t.Helper()
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, code, ce.Code())
	require.Equal(t, token, ce.Message())
}

func TestCreateBatch_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := batchsvc.NewBatchService(gormDB)

	// No principal in ctx -> auth.MustPrincipal returns CodeUnauthenticated.
	_, err := svc.CreateBatch(context.Background(), connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:  "00000000-0000-0000-0000-000000000000",
		ExpiryDate: "2030-06-30",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

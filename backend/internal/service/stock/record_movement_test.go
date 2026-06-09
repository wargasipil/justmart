package stock_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/servicetest"
	stocksvc "github.com/justmart/backend/internal/service/stock"
)

// mainWarehouseID is the migration-seeded default warehouse ("MAIN" / "Gudang
// Utama"). common.ResolveWarehouse falls back to it for an OWNER with no
// WarehouseID, so every PURCHASE seed + handler call below lands here.
const mainWarehouseID = "00000000-0000-0000-0000-0000000000a1"

var skuCounter atomic.Int64

// seedProduct inserts a products row directly via GORM (the SQLite UUID
// create-callback fills the id) and returns its id. SKU is unique per call.
func seedProduct(t *testing.T, db *gorm.DB) string {
	t.Helper()
	p := model.Product{
		SKU:       fmt.Sprintf("stock-test-%d-%d", time.Now().UnixNano(), skuCounter.Add(1)),
		Name:      "Test Product",
		Unit:      "tab",
		UnitPrice: 1000,
		Active:    true,
	}
	require.NoError(t, db.Create(&p).Error)
	require.NotEmpty(t, p.ID)
	return p.ID
}

// seedBatch inserts a global batch lot for productID and returns its id.
func seedBatch(t *testing.T, db *gorm.DB, productID string) string {
	t.Helper()
	b := model.Batch{
		ProductID:   productID,
		BatchNumber: "B-1",
		ExpiryDate:  time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC),
		CostPrice:   500,
		ReceivedAt:  time.Now(),
	}
	require.NoError(t, db.Create(&b).Error)
	require.NotEmpty(t, b.ID)
	return b.ID
}

// seedPurchase inserts a positive PURCHASE movement so a batch has stock in the
// MAIN warehouse, attributed to userID (FK to users.id).
func seedPurchase(t *testing.T, db *gorm.DB, batchID, userID string, qty int32) {
	t.Helper()
	mv := model.StockMovement{
		BatchID:     batchID,
		Qty:         qty,
		Type:        "PURCHASE",
		Reason:      "seed",
		UserID:      userID,
		WarehouseID: mainWarehouseID,
	}
	require.NoError(t, db.Create(&mv).Error)
}

func TestRecordMovement_AdjustmentHappyPath(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := stocksvc.NewStockService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	productID := seedProduct(t, gormDB)
	batchID := seedBatch(t, gormDB, productID)
	seedPurchase(t, gormDB, batchID, ownerID, 10) // 10 on hand in MAIN

	// Positive ADJUSTMENT of +5 -> stock = 15 (>= 0), so it commits.
	resp, err := svc.RecordMovement(ctx, connect.NewRequest(&inventoryifacev1.RecordMovementRequest{
		BatchId: batchID,
		Qty:     5,
		Type:    inventoryifacev1.MovementType_MOVEMENT_TYPE_ADJUSTMENT,
		Reason:  "recount",
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Movement)
	m := resp.Msg.Movement
	require.NotEmpty(t, m.Id)
	require.Equal(t, batchID, m.BatchId)
	require.Equal(t, int32(5), m.Qty)
	require.Equal(t, inventoryifacev1.MovementType_MOVEMENT_TYPE_ADJUSTMENT, m.Type)
	require.Equal(t, "recount", m.Reason)
	require.Equal(t, ownerID, m.UserId)
}

func TestRecordMovement_WriteOffNegativeGuard(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := stocksvc.NewStockService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	productID := seedProduct(t, gormDB)
	batchID := seedBatch(t, gormDB, productID)
	seedPurchase(t, gormDB, batchID, ownerID, 3) // only 3 on hand

	// WRITE_OFF of -10 would drive stock negative -> FailedPrecondition + rollback.
	_, err := svc.RecordMovement(ctx, connect.NewRequest(&inventoryifacev1.RecordMovementRequest{
		BatchId: batchID,
		Qty:     -10,
		Type:    inventoryifacev1.MovementType_MOVEMENT_TYPE_WRITE_OFF,
		Reason:  "damaged",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	// The rolled-back movement must not be persisted: only the seed PURCHASE remains.
	var count int64
	require.NoError(t, gormDB.Model(&model.StockMovement{}).
		Where("batch_id = ?", batchID).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestRecordMovement_InvalidType(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := stocksvc.NewStockService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	productID := seedProduct(t, gormDB)
	batchID := seedBatch(t, gormDB, productID)

	// PURCHASE is rejected — RecordMovement only allows ADJUSTMENT / WRITE_OFF.
	_, err := svc.RecordMovement(ctx, connect.NewRequest(&inventoryifacev1.RecordMovementRequest{
		BatchId: batchID,
		Qty:     5,
		Type:    inventoryifacev1.MovementType_MOVEMENT_TYPE_PURCHASE,
		Reason:  "x",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRecordMovement_MissingBatchID(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := stocksvc.NewStockService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.RecordMovement(ctx, connect.NewRequest(&inventoryifacev1.RecordMovementRequest{
		Qty:  5,
		Type: inventoryifacev1.MovementType_MOVEMENT_TYPE_ADJUSTMENT,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRecordMovement_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := stocksvc.NewStockService(gormDB)

	// No principal in ctx -> auth.MustPrincipal returns CodeUnauthenticated.
	_, err := svc.RecordMovement(context.Background(), connect.NewRequest(&inventoryifacev1.RecordMovementRequest{
		BatchId: "00000000-0000-0000-0000-000000000001",
		Qty:     5,
		Type:    inventoryifacev1.MovementType_MOVEMENT_TYPE_ADJUSTMENT,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

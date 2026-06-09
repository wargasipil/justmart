package transfer_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/servicetest"
	transfersvc "github.com/justmart/backend/internal/service/transfer"
)

// defaultWarehouseID returns the migration-seeded default ("MAIN") warehouse id
// — the one ResolveWarehouse falls back to. CreateTransfer stamps source/dest
// from explicit ids, so a happy-path test needs the real MAIN id as its source.
func defaultWarehouseID(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var wh model.Warehouse
	require.NoError(t, db.Where("is_default").First(&wh).Error)
	return wh.ID
}

// seedWarehouse inserts an active, non-default warehouse and returns its id.
func seedWarehouse(t *testing.T, db *gorm.DB, code, name string) string {
	t.Helper()
	wh := model.Warehouse{Code: code, Name: name, Active: true}
	require.NoError(t, db.Create(&wh).Error)
	return wh.ID
}

// seedStockedBatch creates a product + batch and a PURCHASE movement that puts
// `qty` units of that batch into `warehouseID`. Returns the batch id. This is
// the minimal recipe for a transfer's source-availability check to pass.
func seedStockedBatch(t *testing.T, db *gorm.DB, ownerID, warehouseID string, qty int32) string {
	t.Helper()
	prod := model.Product{
		SKU:       "SKU-" + time.Now().Format("150405.000000"),
		Name:      "Paracetamol 500mg",
		Unit:      "tablet",
		UnitPrice: 1000,
		Active:    true,
	}
	require.NoError(t, db.Create(&prod).Error)
	batch := model.Batch{
		ProductID:   prod.ID,
		BatchNumber: "B-001",
		ExpiryDate:  time.Now().AddDate(1, 0, 0),
		CostPrice:   500,
		ReceivedAt:  time.Now(),
	}
	require.NoError(t, db.Create(&batch).Error)
	mv := model.StockMovement{
		BatchID:     batch.ID,
		Qty:         qty,
		Type:        "PURCHASE",
		Reason:      "seed",
		UserID:      ownerID,
		WarehouseID: warehouseID,
	}
	require.NoError(t, db.Create(&mv).Error)
	return batch.ID
}

func TestCreateTransfer_HappyPath(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := transfersvc.NewTransferService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	from := defaultWarehouseID(t, gormDB)
	to := seedWarehouse(t, gormDB, "WH2", "Gudang Cabang")
	batchID := seedStockedBatch(t, gormDB, ownerID, from, 100)

	resp, err := svc.CreateTransfer(ctx, connect.NewRequest(&warehouseifacev1.CreateTransferRequest{
		FromWarehouseId: from,
		ToWarehouseId:   to,
		Note:            "monthly resupply",
		Lines: []*warehouseifacev1.CreateTransferLineInput{
			{BatchId: batchID, Qty: 40},
		},
	}))
	require.NoError(t, err)
	tr := resp.Msg.Transfer
	require.NotNil(t, tr)
	require.NotEmpty(t, tr.Id)
	require.NotEmpty(t, tr.TransferNo) // TRF-YYYY-NNNN
	require.Equal(t, from, tr.FromWarehouseId)
	require.Equal(t, to, tr.ToWarehouseId)
	require.Equal(t, "monthly resupply", tr.Note)
	require.Equal(t, ownerID, tr.CreatedBy)
	require.Equal(t, "Gudang Cabang", tr.ToWarehouseName) // hydrated
	require.Len(t, tr.Lines, 1)
	require.Equal(t, batchID, tr.Lines[0].BatchId)
	require.Equal(t, int32(40), tr.Lines[0].Qty)
	require.Equal(t, "Paracetamol 500mg", tr.Lines[0].ProductName)

	// The ledger now has a TRANSFER_OUT(-40)@from + TRANSFER_IN(+40)@to pair.
	var outQty, inQty int64
	require.NoError(t, gormDB.Model(&model.StockMovement{}).
		Where("transfer_id = ? AND warehouse_id = ?", tr.Id, from).
		Select("COALESCE(SUM(qty),0)").Scan(&outQty).Error)
	require.NoError(t, gormDB.Model(&model.StockMovement{}).
		Where("transfer_id = ? AND warehouse_id = ?", tr.Id, to).
		Select("COALESCE(SUM(qty),0)").Scan(&inQty).Error)
	require.Equal(t, int64(-40), outQty)
	require.Equal(t, int64(40), inQty)
}

func TestCreateTransfer_InsufficientStock(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := transfersvc.NewTransferService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	from := defaultWarehouseID(t, gormDB)
	to := seedWarehouse(t, gormDB, "WH2", "Gudang Cabang")
	batchID := seedStockedBatch(t, gormDB, ownerID, from, 10) // only 10 on hand

	_, err := svc.CreateTransfer(ctx, connect.NewRequest(&warehouseifacev1.CreateTransferRequest{
		FromWarehouseId: from,
		ToWarehouseId:   to,
		Lines: []*warehouseifacev1.CreateTransferLineInput{
			{BatchId: batchID, Qty: 50}, // more than available
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestCreateTransfer_SameSourceAndDest(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := transfersvc.NewTransferService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	from := defaultWarehouseID(t, gormDB)
	_, err := svc.CreateTransfer(ctx, connect.NewRequest(&warehouseifacev1.CreateTransferRequest{
		FromWarehouseId: from,
		ToWarehouseId:   from, // same -> InvalidArgument
		Lines: []*warehouseifacev1.CreateTransferLineInput{
			{BatchId: "00000000-0000-0000-0000-000000000000", Qty: 1},
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateTransfer_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := transfersvc.NewTransferService(gormDB)

	_, err := svc.CreateTransfer(context.Background(), connect.NewRequest(&warehouseifacev1.CreateTransferRequest{
		FromWarehouseId: "a",
		ToWarehouseId:   "b",
		Lines:           []*warehouseifacev1.CreateTransferLineInput{{BatchId: "x", Qty: 1}},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

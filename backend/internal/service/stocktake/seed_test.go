package stocktake_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/servicetest"
	stocktakesvc "github.com/justmart/backend/internal/service/stocktake"
)

// newSvc builds a StocktakeService against a fresh throwaway SQLite DB plus a
// logged-in OWNER context resolving to the migration-seeded MAIN warehouse.
func newSvc(t *testing.T) (*stocktakesvc.StocktakeService, *gorm.DB, context.Context, string) {
	t.Helper()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	return stocktakesvc.NewStocktakeService(gormDB), gormDB, ctx, ownerID
}

// defaultWarehouseID returns the seeded MAIN (is_default) warehouse id.
func defaultWarehouseID(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var wh model.Warehouse
	require.NoError(t, db.Where("is_default").First(&wh).Error)
	return wh.ID
}

// seedBatchWithStock inserts a product + a batch and a PURCHASE stock movement of
// `qty` base units into `warehouseID`, returning the batch id. The SQLite UUID
// create-callback fills the primary keys.
func seedBatchWithStock(t *testing.T, db *gorm.DB, ownerID, warehouseID string, qty int32) string {
	t.Helper()
	p := model.Product{
		SKU:       "SKU-" + randToken(),
		Name:      "Paracetamol",
		Unit:      "tablet",
		UnitPrice: 1000,
		Active:    true,
	}
	require.NoError(t, db.Create(&p).Error)

	b := model.Batch{
		ProductID:   p.ID,
		BatchNumber: "BATCH-" + randToken(),
		ExpiryDate:  time.Now().AddDate(1, 0, 0),
		CostPrice:   500,
		ReceivedAt:  time.Now(),
	}
	require.NoError(t, db.Create(&b).Error)

	// qty == 0 means "no net stock": leave the batch with no movements (the
	// stock_movements CHECK forbids a zero-qty row, so we can't insert one).
	if qty != 0 {
		mv := model.StockMovement{
			BatchID:     b.ID,
			Qty:         qty,
			Type:        "PURCHASE",
			Reason:      "seed",
			UserID:      ownerID,
			WarehouseID: warehouseID,
		}
		require.NoError(t, db.Create(&mv).Error)
	}
	return b.ID
}

// startDraft starts a fresh DRAFT session and returns its id.
func startDraft(t *testing.T, svc *stocktakesvc.StocktakeService, ctx context.Context, name string) string {
	t.Helper()
	resp, err := svc.StartStocktake(ctx, connect.NewRequest(&stocktakeifacev1.StartStocktakeRequest{Name: name}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Session)
	require.NotEmpty(t, resp.Msg.Session.Id)
	return resp.Msg.Session.Id
}

// addBatch adds one batch to a session and returns the created line id (looked up
// directly so callers don't need the line back from the RPC, which only returns
// counts).
func addBatch(t *testing.T, svc *stocktakesvc.StocktakeService, db *gorm.DB, ctx context.Context, sessionID, batchID string) string {
	t.Helper()
	resp, err := svc.AddBatchesToSession(ctx, connect.NewRequest(&stocktakeifacev1.AddBatchesToSessionRequest{
		SessionId: sessionID,
		BatchIds:  []string{batchID},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.AddedCount)

	var line model.StocktakeLine
	require.NoError(t, db.Where("session_id = ? AND batch_id = ?", sessionID, batchID).First(&line).Error)
	return line.ID
}

// randToken returns a short unique token for SKU/batch-number uniqueness (safe
// under t.Parallel()).
func randToken() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

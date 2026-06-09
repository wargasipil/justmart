package stock_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
	stocksvc "github.com/justmart/backend/internal/service/stock"
)

func TestListMovements_HappyPath(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := stocksvc.NewStockService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	productID := seedProduct(t, gormDB)
	batchID := seedBatch(t, gormDB, productID)
	seedPurchase(t, gormDB, batchID, ownerID, 10)

	// Add one ADJUSTMENT via the handler so there are two movements in MAIN.
	_, err := svc.RecordMovement(ctx, connect.NewRequest(&inventoryifacev1.RecordMovementRequest{
		BatchId: batchID,
		Qty:     2,
		Type:    inventoryifacev1.MovementType_MOVEMENT_TYPE_ADJUSTMENT,
		Reason:  "recount",
	}))
	require.NoError(t, err)

	resp, err := svc.ListMovements(ctx, connect.NewRequest(&inventoryifacev1.ListMovementsRequest{
		BatchId: batchID,
		Limit:   50,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(2), resp.Msg.Total) // PURCHASE seed + the ADJUSTMENT
	require.Len(t, resp.Msg.Movements, 2)
	for _, m := range resp.Msg.Movements {
		require.Equal(t, batchID, m.BatchId)
	}
}

func TestListMovements_TypeFilter(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := stocksvc.NewStockService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	productID := seedProduct(t, gormDB)
	batchID := seedBatch(t, gormDB, productID)
	seedPurchase(t, gormDB, batchID, ownerID, 10)
	_, err := svc.RecordMovement(ctx, connect.NewRequest(&inventoryifacev1.RecordMovementRequest{
		BatchId: batchID,
		Qty:     2,
		Type:    inventoryifacev1.MovementType_MOVEMENT_TYPE_ADJUSTMENT,
	}))
	require.NoError(t, err)

	// Filter to ADJUSTMENT only -> excludes the PURCHASE seed.
	resp, err := svc.ListMovements(ctx, connect.NewRequest(&inventoryifacev1.ListMovementsRequest{
		BatchId: batchID,
		Type:    inventoryifacev1.MovementType_MOVEMENT_TYPE_ADJUSTMENT,
		Limit:   50,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.Len(t, resp.Msg.Movements, 1)
	require.Equal(t, inventoryifacev1.MovementType_MOVEMENT_TYPE_ADJUSTMENT, resp.Msg.Movements[0].Type)
}

func TestListMovements_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := stocksvc.NewStockService(gormDB)

	// No principal in ctx -> auth.MustPrincipal returns CodeUnauthenticated.
	_, err := svc.ListMovements(context.Background(), connect.NewRequest(&inventoryifacev1.ListMovementsRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

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

func TestGetStockLevels_HappyPath(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := stocksvc.NewStockService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	productID := seedProduct(t, gormDB)
	batchID := seedBatch(t, gormDB, productID)
	seedPurchase(t, gormDB, batchID, ownerID, 12) // 12 on hand in MAIN

	resp, err := svc.GetStockLevels(ctx, connect.NewRequest(&inventoryifacev1.GetStockLevelsRequest{
		ProductId: productID,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Levels, 1)
	lvl := resp.Msg.Levels[0]
	require.Equal(t, batchID, lvl.BatchId)
	require.Equal(t, productID, lvl.ProductId)
	require.Equal(t, int64(12), lvl.CurrentQuantity)
	require.NotEmpty(t, lvl.ExpiryDate)
}

func TestGetStockLevels_UnknownProductEmpty(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := stocksvc.NewStockService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Filtering by a product with no batches returns an empty (non-nil) list.
	resp, err := svc.GetStockLevels(ctx, connect.NewRequest(&inventoryifacev1.GetStockLevelsRequest{
		ProductId: "00000000-0000-0000-0000-0000000000ff",
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Levels)
	require.Empty(t, resp.Msg.Levels)
}

func TestGetStockLevels_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := stocksvc.NewStockService(gormDB)

	// No principal in ctx -> auth.MustPrincipal returns CodeUnauthenticated.
	_, err := svc.GetStockLevels(context.Background(), connect.NewRequest(&inventoryifacev1.GetStockLevelsRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

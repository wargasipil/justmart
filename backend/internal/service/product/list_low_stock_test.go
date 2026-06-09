package product_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	productsvc "github.com/justmart/backend/internal/service/product"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestListLowStock_ReturnsBelowThreshold(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	whID := defaultWarehouseID(t, gormDB)

	// Default threshold is 10. One product well-stocked, one below threshold.
	lowID := seedProduct(t, svc, ctx, "SKU-LOW-1", "LowStockMed", 1000)
	seedBatchWithStock(t, gormDB, lowID, whID, ownerID, 3, 500) // 3 <= 10 -> low

	highID := seedProduct(t, svc, ctx, "SKU-LOW-2", "WellStockedMed", 1000)
	seedBatchWithStock(t, gormDB, highID, whID, ownerID, 50, 500) // 50 > 10 -> not low

	resp, err := svc.ListLowStock(ctx, connect.NewRequest(&inventoryifacev1.ListLowStockRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(10), resp.Msg.Threshold) // default when no app_settings row

	got := map[string]*inventoryifacev1.Product{}
	for _, p := range resp.Msg.Products {
		got[p.Id] = p
	}
	require.Contains(t, got, lowID)
	require.Equal(t, int64(3), got[lowID].ReadyStock)
	require.NotContains(t, got, highID) // above threshold, excluded
}

func TestListLowStock_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := productsvc.NewProductService(gormDB)

	_, err := svc.ListLowStock(context.Background(), connect.NewRequest(&inventoryifacev1.ListLowStockRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

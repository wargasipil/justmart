package product_test

import (
	"context"
	"fmt"
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

// `total` is the FULL match count, not the page length — the TopBar bell badge
// reads it, and it previously reported len(products) under a hard Limit(100).
func TestListLowStock_PaginatesAndTotalIgnoresWindow(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	whID := defaultWarehouseID(t, gormDB)

	// 5 products under the default threshold of 10.
	for i := 0; i < 5; i++ {
		pid := seedProduct(t, svc, ctx, fmt.Sprintf("SKU-LOWPAGE-%d", i), fmt.Sprintf("LowPaged %d", i), 1000)
		seedBatchWithStock(t, gormDB, pid, whID, ownerID, int32(i+1), 500)
	}

	page := func(limit, offset int32) *inventoryifacev1.ListLowStockResponse {
		t.Helper()
		resp, err := svc.ListLowStock(ctx, connect.NewRequest(
			&inventoryifacev1.ListLowStockRequest{Limit: limit, Offset: offset}))
		require.NoError(t, err)
		return resp.Msg
	}

	first := page(2, 0)
	require.Equal(t, int32(5), first.Total) // the badge count, not the page size
	require.Len(t, first.Products, 2)
	require.Equal(t, int32(10), first.Threshold)
	// Ordered by ready ASC, so the emptiest lands first.
	require.Equal(t, int64(1), first.Products[0].ReadyStock)

	seen := map[string]bool{}
	for off := int32(0); off < first.Total; off += 2 {
		pg := page(2, off)
		require.Equal(t, first.Total, pg.Total)
		for _, p := range pg.Products {
			require.False(t, seen[p.Id], "product %s appeared on two pages", p.Sku)
			seen[p.Id] = true
		}
	}
	require.Len(t, seen, 5)
}

func TestListLowStock_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := productsvc.NewProductService(gormDB)

	_, err := svc.ListLowStock(context.Background(), connect.NewRequest(&inventoryifacev1.ListLowStockRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

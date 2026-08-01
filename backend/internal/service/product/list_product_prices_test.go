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

func TestListProductPrices_ReturnsHistory(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	pid := seedProduct(t, svc, ctx, "SKU-PRC-1", "PricedMed", 1000)
	// Change price -> a second price-history row gets created.
	_, err := svc.UpdateProduct(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
		Id:        pid,
		Name:      "PricedMed",
		Unit:      "tablet",
		UnitPrice: 1750,
	}))
	require.NoError(t, err)

	resp, err := svc.ListProductPrices(ctx, connect.NewRequest(&inventoryifacev1.ListProductPricesRequest{
		ProductId: pid,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Prices, 2)
	// Ordered effective_from DESC -> newest (open) row first.
	require.Equal(t, int64(1750), resp.Msg.Prices[0].UnitPrice)
	require.Equal(t, int64(0), resp.Msg.Prices[0].EffectiveTo) // open row
	require.Equal(t, int64(1000), resp.Msg.Prices[1].UnitPrice)
	require.NotZero(t, resp.Msg.Prices[1].EffectiveTo) // closed row
}

func TestListProductPrices_Paginates(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	pid := seedProduct(t, svc, ctx, "SKU-PRC-PAGE", "PagedPriceMed", 1000)
	for _, p := range []int64{1100, 1200, 1300, 1400} {
		_, err := svc.UpdateProduct(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
			Id: pid, Name: "PagedPriceMed", Unit: "tablet", UnitPrice: p,
		}))
		require.NoError(t, err)
	}

	page := func(limit, offset int32) *inventoryifacev1.ListProductPricesResponse {
		t.Helper()
		resp, err := svc.ListProductPrices(ctx, connect.NewRequest(
			&inventoryifacev1.ListProductPricesRequest{ProductId: pid, Limit: limit, Offset: offset}))
		require.NoError(t, err)
		return resp.Msg
	}

	first := page(2, 0)
	require.Equal(t, int32(5), first.Total) // seed + 4 edits
	require.Len(t, first.Prices, 2)
	require.Equal(t, int64(1400), first.Prices[0].UnitPrice) // newest first

	seen := map[string]bool{}
	for off := int32(0); off < first.Total; off += 2 {
		pg := page(2, off)
		require.Equal(t, first.Total, pg.Total)
		for _, p := range pg.Prices {
			require.False(t, seen[p.Id], "price row %s appeared on two pages", p.Id)
			seen[p.Id] = true
		}
	}
	require.Len(t, seen, 5)
}

func TestListProductPrices_MissingProductID(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := productsvc.NewProductService(gormDB)

	_, err := svc.ListProductPrices(context.Background(), connect.NewRequest(&inventoryifacev1.ListProductPricesRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

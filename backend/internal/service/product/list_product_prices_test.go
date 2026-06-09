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

func TestListProductPrices_MissingProductID(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := productsvc.NewProductService(gormDB)

	_, err := svc.ListProductPrices(context.Background(), connect.NewRequest(&inventoryifacev1.ListProductPricesRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

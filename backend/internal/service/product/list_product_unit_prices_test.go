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

func TestListProductUnitPrices_ReturnsBaseUnitHistory(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Create with an extra non-base unit so the per-unit price history is richer.
	resp, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku:       "SKU-UPRC-1",
		Name:      "MultiUnitMed",
		Unit:      "tablet",
		UnitPrice: 1000,
		Units: []*inventoryifacev1.ProductUnitInput{
			{Name: "box", Factor: 100, SellPrice: 90000, Sellable: true, Purchasable: true},
		},
	}))
	require.NoError(t, err)
	pid := resp.Msg.Product.Id

	pricesResp, err := svc.ListProductUnitPrices(ctx, connect.NewRequest(&inventoryifacev1.ListProductUnitPricesRequest{
		ProductId: pid,
	}))
	require.NoError(t, err)
	// One seeded price row per unit (base + box).
	require.Len(t, pricesResp.Msg.Prices, 2)
	// Ordered base-first.
	require.Equal(t, "tablet", pricesResp.Msg.Prices[0].UnitName)
	require.Equal(t, int64(1000), pricesResp.Msg.Prices[0].UnitSellPrice)
	require.Equal(t, "box", pricesResp.Msg.Prices[1].UnitName)
	require.Equal(t, int64(90000), pricesResp.Msg.Prices[1].UnitSellPrice)
}

func TestListProductUnitPrices_MissingProductID(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := productsvc.NewProductService(gormDB)

	_, err := svc.ListProductUnitPrices(context.Background(), connect.NewRequest(&inventoryifacev1.ListProductUnitPricesRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

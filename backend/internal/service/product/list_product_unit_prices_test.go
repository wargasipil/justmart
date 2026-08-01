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

// Price history is the one product-detail read that grows without bound (a row
// per price edit per unit), so paging has to walk it exactly once.
func TestListProductUnitPrices_Paginates(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	resp, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: "SKU-UPRC-PAGE", Name: "PagedPrices", Unit: "tablet", UnitPrice: 1000,
	}))
	require.NoError(t, err)
	pid := resp.Msg.Product.Id

	// Each price edit closes the open row and opens a new one -> history grows.
	for _, p := range []int64{1100, 1200, 1300, 1400} {
		_, err := svc.UpdateProduct(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
			Id: pid, Name: "PagedPrices", Unit: "tablet", UnitPrice: p,
		}))
		require.NoError(t, err)
	}

	page := func(limit, offset int32) *inventoryifacev1.ListProductUnitPricesResponse {
		t.Helper()
		r, err := svc.ListProductUnitPrices(ctx, connect.NewRequest(
			&inventoryifacev1.ListProductUnitPricesRequest{
				ProductId: pid, Limit: limit, Offset: offset,
			}))
		require.NoError(t, err)
		return r.Msg
	}

	all := page(100, 0)
	require.Equal(t, int32(5), all.Total) // seed + 4 edits
	require.Len(t, all.Prices, 5)

	// Walk it two at a time; every row appears exactly once.
	seen := map[string]bool{}
	for off := int32(0); off < all.Total; off += 2 {
		pg := page(2, off)
		require.Equal(t, all.Total, pg.Total) // total ignores the window
		for _, p := range pg.Prices {
			require.False(t, seen[p.Id], "price row %s appeared on two pages", p.Id)
			seen[p.Id] = true
		}
	}
	require.Len(t, seen, 5)
}

func TestListProductUnitPrices_MissingProductID(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := productsvc.NewProductService(gormDB)

	_, err := svc.ListProductUnitPrices(context.Background(), connect.NewRequest(&inventoryifacev1.ListProductUnitPricesRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

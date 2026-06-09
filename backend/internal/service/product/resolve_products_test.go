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

func TestResolveProducts_ReturnsRefs(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	id1 := seedProduct(t, svc, ctx, "SKU-RES-1", "Metformin 500mg", 1000)
	id2 := seedProduct(t, svc, ctx, "SKU-RES-2", "Glibenclamide 5mg", 1500)

	// Include one unknown id — it must be omitted, not error.
	resp, err := svc.ResolveProducts(ctx, connect.NewRequest(&inventoryifacev1.ResolveProductsRequest{
		Ids: []string{id1, id2, "00000000-0000-0000-0000-000000000000"},
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Products, 2)

	byID := map[string]*inventoryifacev1.ProductRef{}
	for _, r := range resp.Msg.Products {
		byID[r.Id] = r
	}
	require.Equal(t, "Metformin 500mg", byID[id1].Name)
	require.Equal(t, "SKU-RES-1", byID[id1].Sku)
	require.Equal(t, "Glibenclamide 5mg", byID[id2].Name)
}

func TestResolveProducts_EmptyInput(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := productsvc.NewProductService(gormDB)

	resp, err := svc.ResolveProducts(context.Background(), connect.NewRequest(&inventoryifacev1.ResolveProductsRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Products)
}

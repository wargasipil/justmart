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

func TestSearchProducts_MatchesNameOrSku(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, svc, ctx, "SKU-SRCH-1", "Omeprazole 20mg", 3000)
	seedProduct(t, svc, ctx, "SKU-SRCH-2", "Ranitidine 150mg", 2500)

	resp, err := svc.SearchProducts(ctx, connect.NewRequest(&inventoryifacev1.SearchProductsRequest{
		Query: "Omepra",
		Limit: 10,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Products, 1)
	require.Equal(t, "Omeprazole 20mg", resp.Msg.Products[0].Name)
	require.NotEmpty(t, resp.Msg.Products[0].Units) // attachUnits ran
}

func TestSearchProducts_EmptyQueryReturnsAll(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, svc, ctx, "SKU-SRCH-A", "Vitamin C", 500)

	// Empty query -> no ILIKE; returns active products (capped at the default 20).
	resp, err := svc.SearchProducts(ctx, connect.NewRequest(&inventoryifacev1.SearchProductsRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Products, 1)
}

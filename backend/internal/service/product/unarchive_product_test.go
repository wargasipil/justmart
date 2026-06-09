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

func TestUnarchiveProduct_SetsActive(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	pid := seedProduct(t, svc, ctx, "SKU-UNARCH-1", "ToUnarchive", 1000)
	// Archive first so unarchive has a real effect.
	_, err := svc.ArchiveProduct(ctx, connect.NewRequest(&inventoryifacev1.ArchiveProductRequest{Id: pid}))
	require.NoError(t, err)

	resp, err := svc.UnarchiveProduct(ctx, connect.NewRequest(&inventoryifacev1.UnarchiveProductRequest{Id: pid}))
	require.NoError(t, err)
	require.Equal(t, pid, resp.Msg.Product.Id)
	require.True(t, resp.Msg.Product.Active)
}

func TestUnarchiveProduct_NotFound(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.UnarchiveProduct(ctx, connect.NewRequest(&inventoryifacev1.UnarchiveProductRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

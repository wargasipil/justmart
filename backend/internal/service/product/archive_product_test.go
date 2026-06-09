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

func TestArchiveProduct_SetsInactive(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	pid := seedProduct(t, svc, ctx, "SKU-ARCH-1", "ToArchive", 1000)

	resp, err := svc.ArchiveProduct(ctx, connect.NewRequest(&inventoryifacev1.ArchiveProductRequest{Id: pid}))
	require.NoError(t, err)
	require.Equal(t, pid, resp.Msg.Product.Id)
	require.False(t, resp.Msg.Product.Active)
}

func TestArchiveProduct_NotFound(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.ArchiveProduct(ctx, connect.NewRequest(&inventoryifacev1.ArchiveProductRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

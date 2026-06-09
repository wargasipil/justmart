package product_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	productsvc "github.com/justmart/backend/internal/service/product"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestUpdateProduct_ChangesNameAndPrice(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	pid := seedProduct(t, svc, ctx, "SKU-UPD-1", "Old Name", 1000)

	resp, err := svc.UpdateProduct(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
		Id:                   pid,
		Name:                 "New Name",
		Unit:                 "tablet",
		UnitPrice:            1800,
		PrescriptionRequired: true,
	}))
	require.NoError(t, err)
	p := resp.Msg.Product
	require.Equal(t, "New Name", p.Name)
	require.Equal(t, int64(1800), p.UnitPrice)
	require.True(t, p.PrescriptionRequired)

	// A price change closes the old open row and opens a new one -> 2 history rows.
	var openCount int64
	require.NoError(t, gormDB.Model(&model.ProductPrice{}).
		Where("product_id = ? AND effective_to IS NULL", pid).Count(&openCount).Error)
	require.Equal(t, int64(1), openCount) // exactly one open row at all times

	var totalCount int64
	require.NoError(t, gormDB.Model(&model.ProductPrice{}).
		Where("product_id = ?", pid).Count(&totalCount).Error)
	require.Equal(t, int64(2), totalCount)
}

func TestUpdateProduct_MissingName(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	pid := seedProduct(t, svc, ctx, "SKU-UPD-2", "Keepme", 1000)

	_, err := svc.UpdateProduct(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
		Id:   pid,
		Name: "  ",
		Unit: "tablet",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpdateProduct_NotFound(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.UpdateProduct(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
		Id:        "00000000-0000-0000-0000-000000000000",
		Name:      "x",
		Unit:      "tablet",
		UnitPrice: 1,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

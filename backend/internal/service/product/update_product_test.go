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
		Sku:                  "SKU-UPD-1", // unchanged (SKU is required on update)
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

func TestUpdateProduct_ChangesSKU(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	pid := seedProduct(t, svc, ctx, "SKU-OLD", "Product", 1000)

	resp, err := svc.UpdateProduct(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
		Id: pid, Sku: "SKU-NEW", Name: "Product", Unit: "tablet", UnitPrice: 1000,
	}))
	require.NoError(t, err)
	require.Equal(t, "SKU-NEW", resp.Msg.Product.Sku)

	// Persisted to the row.
	var p model.Product
	require.NoError(t, gormDB.First(&p, "id = ?", pid).Error)
	require.Equal(t, "SKU-NEW", p.SKU)
}

func TestUpdateProduct_DuplicateSKURejected(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, svc, ctx, "SKU-A", "A", 1000)
	b := seedProduct(t, svc, ctx, "SKU-B", "B", 1000)

	// Renaming B onto A's SKU is rejected with the same token as create.
	_, err := svc.UpdateProduct(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
		Id: b, Sku: "SKU-A", Name: "B", Unit: "tablet", UnitPrice: 1000,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, "product.sku_taken", ce.Message())
}

func TestUpdateProduct_KeepsOwnSKU(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	pid := seedProduct(t, svc, ctx, "SKU-SELF", "Old", 1000)

	// Resubmitting the row's own SKU while editing other fields must NOT trip
	// the uniqueness check (id <> ? excludes self).
	resp, err := svc.UpdateProduct(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
		Id: pid, Sku: "SKU-SELF", Name: "New", Unit: "tablet", UnitPrice: 1200,
	}))
	require.NoError(t, err)
	require.Equal(t, "SKU-SELF", resp.Msg.Product.Sku)
	require.Equal(t, "New", resp.Msg.Product.Name)
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

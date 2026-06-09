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

func TestCreateProduct_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	resp, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku:                  "SKU-AMOX-500",
		Name:                 "Amoxicillin 500mg",
		Unit:                 "tablet",
		UnitPrice:            1500,
		PrescriptionRequired: true,
	}))
	require.NoError(t, err)
	p := resp.Msg.Product
	require.NotNil(t, p)
	require.NotEmpty(t, p.Id) // UUID filled by the SQLite create-callback
	require.Equal(t, "SKU-AMOX-500", p.Sku)
	require.Equal(t, "Amoxicillin 500mg", p.Name)
	require.Equal(t, "tablet", p.Unit)
	require.Equal(t, int64(1500), p.UnitPrice)
	require.True(t, p.PrescriptionRequired)
	require.True(t, p.Active)
	// A base unit (factor 1) is synced on create.
	require.Len(t, p.Units, 1)
	require.Equal(t, "tablet", p.Units[0].Name)
	require.True(t, p.Units[0].IsBase)
	require.Equal(t, int64(1), p.Units[0].Factor)
}

func TestCreateProduct_MissingRequiredFields(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Empty name (trimmed to "") -> InvalidArgument.
	_, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku:  "SKU-X",
		Name: "   ",
		Unit: "tablet",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateProduct_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := productsvc.NewProductService(gormDB)

	_, err := svc.CreateProduct(context.Background(), connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku:  "SKU-Y",
		Name: "Paracetamol",
		Unit: "tablet",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

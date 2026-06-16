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

func importRow(sku, name, unit string, price int64) *inventoryifacev1.CreateProductRequest {
	return &inventoryifacev1.CreateProductRequest{Sku: sku, Name: name, Unit: unit, UnitPrice: price}
}

func TestImportProducts_HappyPath(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	resp, err := svc.ImportProducts(ctx, connect.NewRequest(&inventoryifacev1.ImportProductsRequest{
		Products: []*inventoryifacev1.CreateProductRequest{
			importRow("IMP-1", "Paracetamol", "tablet", 500),
			importRow("IMP-2", "Amoxicillin", "tablet", 1500),
		},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(2), resp.Msg.Created)
	require.Equal(t, int32(0), resp.Msg.Skipped)
	require.Equal(t, int32(0), resp.Msg.Errored)
	require.Len(t, resp.Msg.Results, 2)
	require.Equal(t, inventoryifacev1.ImportProductStatus_IMPORT_PRODUCT_STATUS_CREATED, resp.Msg.Results[0].Status)
	require.NotEmpty(t, resp.Msg.Results[0].ProductId)

	// Both products are now listable (base unit synced).
	list, err := svc.ListProducts(ctx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(2), list.Msg.Total)
}

func TestImportProducts_SkipsExistingSKU(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Seed an existing product, then import a row with the same SKU + a new one.
	_, err := svc.CreateProduct(ctx, connect.NewRequest(importRow("DUP-1", "Existing", "tablet", 100)))
	require.NoError(t, err)

	resp, err := svc.ImportProducts(ctx, connect.NewRequest(&inventoryifacev1.ImportProductsRequest{
		Products: []*inventoryifacev1.CreateProductRequest{
			importRow("DUP-1", "Existing (reimport)", "tablet", 999),
			importRow("NEW-1", "Brand new", "tablet", 200),
		},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Created)
	require.Equal(t, int32(1), resp.Msg.Skipped)
	require.Equal(t, int32(0), resp.Msg.Errored)
	require.Equal(t, inventoryifacev1.ImportProductStatus_IMPORT_PRODUCT_STATUS_SKIPPED_DUPLICATE, resp.Msg.Results[0].Status)
	require.Equal(t, inventoryifacev1.ImportProductStatus_IMPORT_PRODUCT_STATUS_CREATED, resp.Msg.Results[1].Status)

	// The existing product was NOT overwritten (create-only).
	got, err := svc.ListProducts(ctx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{Query: "DUP-1"}))
	require.NoError(t, err)
	require.Equal(t, int32(1), got.Msg.Total)
	require.Equal(t, "Existing", got.Msg.Products[0].Name)
	require.Equal(t, int64(100), got.Msg.Products[0].UnitPrice)
}

func TestImportProducts_InFileDuplicate(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	resp, err := svc.ImportProducts(ctx, connect.NewRequest(&inventoryifacev1.ImportProductsRequest{
		Products: []*inventoryifacev1.CreateProductRequest{
			importRow("SAME", "First wins", "tablet", 100),
			importRow("SAME", "Second skipped", "tablet", 200),
		},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Created)
	require.Equal(t, int32(1), resp.Msg.Skipped)
	require.Equal(t, inventoryifacev1.ImportProductStatus_IMPORT_PRODUCT_STATUS_CREATED, resp.Msg.Results[0].Status)
	require.Equal(t, inventoryifacev1.ImportProductStatus_IMPORT_PRODUCT_STATUS_SKIPPED_DUPLICATE, resp.Msg.Results[1].Status)
}

func TestImportProducts_PartialSuccess(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	resp, err := svc.ImportProducts(ctx, connect.NewRequest(&inventoryifacev1.ImportProductsRequest{
		Products: []*inventoryifacev1.CreateProductRequest{
			importRow("OK-1", "Valid", "tablet", 500),
			importRow("BAD-1", "  ", "tablet", 100),  // blank name
			importRow("BAD-2", "Neg price", "tablet", -5), // negative price
		},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Created)
	require.Equal(t, int32(0), resp.Msg.Skipped)
	require.Equal(t, int32(2), resp.Msg.Errored)
	require.Equal(t, inventoryifacev1.ImportProductStatus_IMPORT_PRODUCT_STATUS_CREATED, resp.Msg.Results[0].Status)
	require.Equal(t, inventoryifacev1.ImportProductStatus_IMPORT_PRODUCT_STATUS_ERROR, resp.Msg.Results[1].Status)
	require.NotEmpty(t, resp.Msg.Results[1].Message)
	require.Equal(t, inventoryifacev1.ImportProductStatus_IMPORT_PRODUCT_STATUS_ERROR, resp.Msg.Results[2].Status)

	// The one valid row persisted despite the bad rows.
	list, err := svc.ListProducts(ctx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(1), list.Msg.Total)
	require.Equal(t, "OK-1", list.Msg.Products[0].Sku)
}

func TestImportProducts_WithUnits(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// A valid row carrying an extra pack, plus a row whose pack has a bad factor.
	good := importRow("U-OK", "Amoxicillin", "tablet", 1000)
	good.Units = []*inventoryifacev1.ProductUnitInput{
		{Name: "box", Factor: 100, SellPrice: 90000, Sellable: true, Purchasable: true},
		{Name: "strip", Factor: 10, SellPrice: 9500, Sellable: true, Purchasable: true},
	}
	bad := importRow("U-BAD", "Bad pack", "tablet", 1000)
	bad.Units = []*inventoryifacev1.ProductUnitInput{
		{Name: "box", Factor: 1, SellPrice: 5000, Sellable: true, Purchasable: true}, // factor must be > 1
	}

	resp, err := svc.ImportProducts(ctx, connect.NewRequest(&inventoryifacev1.ImportProductsRequest{
		Products: []*inventoryifacev1.CreateProductRequest{good, bad},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Created)
	require.Equal(t, int32(1), resp.Msg.Errored)
	require.Equal(t, inventoryifacev1.ImportProductStatus_IMPORT_PRODUCT_STATUS_CREATED, resp.Msg.Results[0].Status)
	require.Equal(t, inventoryifacev1.ImportProductStatus_IMPORT_PRODUCT_STATUS_ERROR, resp.Msg.Results[1].Status)
	require.NotEmpty(t, resp.Msg.Results[1].Message)

	// The created product has the base unit + the two imported packs.
	got, err := svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{
		Id: resp.Msg.Results[0].ProductId,
	}))
	require.NoError(t, err)
	names := map[string]int64{}
	for _, u := range got.Msg.Product.Units {
		names[u.Name] = u.Factor
	}
	require.Equal(t, int64(1), names["tablet"]) // base
	require.Equal(t, int64(100), names["box"])
	require.Equal(t, int64(10), names["strip"])

	// The bad-pack product was not created (its row rolled back).
	list, err := svc.ListProducts(ctx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{Query: "U-BAD"}))
	require.NoError(t, err)
	require.Equal(t, int32(0), list.Msg.Total)
}

func TestImportProducts_EmptyRejected(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.ImportProducts(ctx, connect.NewRequest(&inventoryifacev1.ImportProductsRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestImportProducts_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := productsvc.NewProductService(gormDB)

	_, err := svc.ImportProducts(context.Background(), connect.NewRequest(&inventoryifacev1.ImportProductsRequest{
		Products: []*inventoryifacev1.CreateProductRequest{importRow("X", "Y", "tablet", 1)},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

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

func newExpiryProductEnv(t *testing.T) (*productsvc.ProductService, context.Context) {
	t.Helper()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	return productsvc.NewProductService(gormDB), servicetest.OwnerCtx(context.Background(), ownerID)
}

func createWithExpiry(t *testing.T, svc *productsvc.ProductService, ctx context.Context,
	sku string, d inventoryifacev1.ExpiryDefault, months int32, rx bool,
) *inventoryifacev1.Product {
	t.Helper()
	resp, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: sku, Name: sku, Unit: "pcs", UnitPrice: 1000,
		PrescriptionRequired: rx,
		ExpiryDefault:        d,
		ExpiryDefaultMonths:  months,
	}))
	require.NoError(t, err)
	return resp.Msg.Product
}

// One light read returns each product's setting plus the prescription flag the
// dialog's pharmacy rule needs; unknown ids are simply absent.
func TestGetProductExpiryDefaults_ReturnsEachSetting(t *testing.T) {
	t.Parallel()
	svc, ctx := newExpiryProductEnv(t)
	months := createWithExpiry(t, svc, ctx, "PED-M", inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_MONTHS, 24, false)
	none := createWithExpiry(t, svc, ctx, "PED-N", inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_NONE, 0, false)
	manual := createWithExpiry(t, svc, ctx, "PED-X", inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_UNSPECIFIED, 0, true)

	resp, err := svc.GetProductExpiryDefaults(ctx, connect.NewRequest(&inventoryifacev1.GetProductExpiryDefaultsRequest{
		ProductIds: []string{months.Id, none.Id, manual.Id, months.Id, "00000000-0000-0000-0000-000000000000"},
	}))
	require.NoError(t, err)
	byID := map[string]*inventoryifacev1.ProductExpiryDefault{}
	for _, d := range resp.Msg.Defaults {
		byID[d.ProductId] = d
	}
	require.Len(t, byID, 3, "deduped, unknown id omitted")

	require.Equal(t, inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_MONTHS, byID[months.Id].ExpiryDefault)
	require.Equal(t, int32(24), byID[months.Id].ExpiryDefaultMonths)
	require.Equal(t, inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_NONE, byID[none.Id].ExpiryDefault)
	require.Equal(t, inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_MANUAL, byID[manual.Id].ExpiryDefault,
		"UNSPECIFIED is stored as MANUAL")
	require.True(t, byID[manual.Id].PrescriptionRequired)
}

func TestGetProductExpiryDefaults_EmptyInput(t *testing.T) {
	t.Parallel()
	svc, ctx := newExpiryProductEnv(t)
	resp, err := svc.GetProductExpiryDefaults(ctx, connect.NewRequest(&inventoryifacev1.GetProductExpiryDefaultsRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Defaults)
}

// The setting is validated on create AND update, and months only survive with
// MONTHS -- a stray number beside NONE is dropped, not stored.
func TestProductExpiryDefault_ValidationAndUpdate(t *testing.T) {
	t.Parallel()
	svc, ctx := newExpiryProductEnv(t)

	for _, m := range []int32{0, 121} {
		_, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
			Sku: "PED-BAD", Name: "Bad", Unit: "pcs",
			ExpiryDefault: inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_MONTHS, ExpiryDefaultMonths: m,
		}))
		var ce *connect.Error
		require.ErrorAs(t, err, &ce, "months=%d", m)
		require.Equal(t, "product.expiry_months_invalid", ce.Message())
	}

	p := createWithExpiry(t, svc, ctx, "PED-U", inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_NONE, 7, false)
	require.Equal(t, inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_NONE, p.ExpiryDefault)
	require.Zero(t, p.ExpiryDefaultMonths, "months are kept only for MONTHS")

	update := func(d inventoryifacev1.ExpiryDefault, months int32) (*inventoryifacev1.Product, error) {
		resp, err := svc.UpdateProduct(ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
			Id: p.Id, Name: p.Name, Unit: "pcs", UnitPrice: 1000,
			ExpiryDefault: d, ExpiryDefaultMonths: months,
		}))
		if err != nil {
			return nil, err
		}
		return resp.Msg.Product, nil
	}
	got, err := update(inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_MONTHS, 18)
	require.NoError(t, err)
	require.Equal(t, inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_MONTHS, got.ExpiryDefault)
	require.Equal(t, int32(18), got.ExpiryDefaultMonths)

	_, err = update(inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_MONTHS, -1)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, "product.expiry_months_invalid", ce.Message())

	// UpdateProduct is a full replace: a form that omits the setting resets it.
	got, err = update(inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_UNSPECIFIED, 0)
	require.NoError(t, err)
	require.Equal(t, inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_MANUAL, got.ExpiryDefault)
}

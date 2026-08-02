package product_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// A COMPOSITE and a SERVICE hold no batches, so the low-stock join scores them 0
// and they would sit in the TopBar bell permanently — in a restaurant that is
// every menu item and every fee, i.e. a badge nobody can ever clear.
//
// Excluding them is also the right domain answer: the bell is about
// REPLENISHABLE stock. A menu item runs low only because an ingredient did, and
// the ingredient IS a stocked product the bell reports.
func TestListLowStock_ExcludesCompositeAndService(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newKindEnv(t)
	whID := defaultWarehouseID(t, db)

	// A genuinely low stocked product — must still be reported.
	low, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: "SKU-LOW", Name: "Beras", Unit: "g", UnitPrice: 100,
	}))
	require.NoError(t, err)
	seedBatchWithStock(t, db, low.Msg.Product.Id, whID, ownerID, 1, 50)

	// A menu item and a fee — both batchless, neither reorderable.
	dish, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: "SKU-MENU", Name: "Nasi Goreng", Unit: "porsi", UnitPrice: 20000,
		Kind: inventoryifacev1.ProductKind_PRODUCT_KIND_COMPOSITE,
	}))
	require.NoError(t, err)
	fee, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: "SKU-FEE", Name: "Corkage", Unit: "x", UnitPrice: 15000,
		Kind: inventoryifacev1.ProductKind_PRODUCT_KIND_SERVICE,
	}))
	require.NoError(t, err)

	resp, err := svc.ListLowStock(ctx, connect.NewRequest(&inventoryifacev1.ListLowStockRequest{}))
	require.NoError(t, err)

	ids := map[string]bool{}
	for _, p := range resp.Msg.Products {
		ids[p.Id] = true
	}
	require.True(t, ids[low.Msg.Product.Id], "a low stocked product must still be reported")
	require.False(t, ids[dish.Msg.Product.Id], "a menu item is not reorderable — it must not fill the bell")
	require.False(t, ids[fee.Msg.Product.Id], "a service has no stock and can never be low")

	// The badge count must agree with the rows, not count the excluded kinds.
	require.Equal(t, int32(len(resp.Msg.Products)), resp.Msg.Total)
}

// A product written before product_kind existed reads as STOCKED, so it keeps
// appearing in the bell exactly as it did.
//
// Note this is asserted on the READ helper rather than by writing '' to a row:
// migration 00054 backfills every existing row to 'STOCKED' via the column
// default, and the Postgres CHECK makes a blank kind unrepresentable altogether
// — only SQLite would accept one. Writing '' would therefore be testing a
// SQLite quirk, not a state production can reach. The COALESCE in the low-stock
// filter and the fallback here are belt-and-braces for exactly that reason.
func TestProductKind_BlankReadsAsStocked(t *testing.T) {
	t.Parallel()
	require.Equal(t, common.ProductKindStocked, common.NormalizeProductKind(""))
	require.Equal(t, common.ProductKindStocked, common.NormalizeProductKind("NONSENSE"))
	require.Equal(t, common.ProductKindComposite, common.NormalizeProductKind(common.ProductKindComposite))
	require.Equal(t, common.ProductKindService, common.NormalizeProductKind(common.ProductKindService))
}

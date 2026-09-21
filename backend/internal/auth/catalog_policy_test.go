package auth_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	// Registers the inventory descriptors with protoregistry.GlobalFiles, which
	// is what BuildPolicy walks. Without these blank imports the map comes back
	// missing the very procedures under test, and every assertion below would
	// fail on a lookup rather than on the policy.
	_ "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/service/common"
)

// The catalog is fully READABLE by the till and still only WRITABLE by managers.
// That split lives in the proto allowed_roles, so this pins it at the layer that
// actually enforces it — the interceptor reads exactly this map. The redactor
// tests (product/batch redact_test.go) cover the response body; this covers who
// may make the call at all.
func TestCatalogPolicy_TillReadsEverythingWritesNothing(t *testing.T) {
	t.Parallel()
	policy := auth.BuildPolicy()

	// Reads a cashier must be able to make. Each one backs a surface on
	// /products or /products/:id; a narrower guard here means a permission
	// toast on page load, not a hidden field.
	reads := []string{
		"/inventory_iface.v1.ProductService/ListProducts",
		"/inventory_iface.v1.ProductService/GetProduct",
		"/inventory_iface.v1.ProductService/GetProductsSummary",
		"/inventory_iface.v1.ProductService/ListProductUnitPrices",
		"/inventory_iface.v1.ProductService/ListProductRestockLogs",
		"/inventory_iface.v1.ProductDiscountService/ListProductDiscounts",
		"/inventory_iface.v1.ProductPriceTierService/ListProductPriceTiers",
		"/inventory_iface.v1.ProductPriceTierService/ListProductTierPrices",
		"/inventory_iface.v1.StockMovementService/ListMovements",
		"/inventory_iface.v1.BatchService/ListBatches",
		"/inventory_iface.v1.BatchService/GetBatch",
		"/inventory_iface.v1.SupplierService/ResolveSuppliers",
	}
	for _, proc := range reads {
		p, ok := policy[proc]
		require.True(t, ok, "missing procedure %s", proc)
		require.False(t, p.Public, proc)
		for _, role := range []string{"OWNER", "PHARMACIST", "CASHIER", "APOTEKER"} {
			require.Contains(t, p.AllowedRoles, role, "%s should be readable by %s", proc, role)
		}
	}

	// Writes the till must NOT be able to make. Cost visibility widened; catalog
	// authorship did not.
	writes := []string{
		"/inventory_iface.v1.ProductService/CreateProduct",
		"/inventory_iface.v1.ProductService/UpdateProduct",
		"/inventory_iface.v1.ProductService/ArchiveProduct",
		"/inventory_iface.v1.ProductService/ImportProducts",
		"/inventory_iface.v1.ProductService/UploadProductImage",
		"/inventory_iface.v1.ProductDiscountService/CreateProductDiscount",
		"/inventory_iface.v1.ProductDiscountService/DeleteProductDiscount",
		"/inventory_iface.v1.ProductPriceTierService/CreateProductPriceTier",
		"/inventory_iface.v1.ProductPriceTierService/DeleteProductPriceTier",
		"/inventory_iface.v1.StockMovementService/RecordMovement",
		// The supplier domain stays manager-only apart from the id->name
		// resolve above; a browsable supplier list is not part of the catalog.
		"/inventory_iface.v1.SupplierService/ListSuppliers",
		"/inventory_iface.v1.SupplierService/SearchSuppliers",
	}
	for _, proc := range writes {
		p, ok := policy[proc]
		require.True(t, ok, "missing procedure %s", proc)
		require.NotContains(t, p.AllowedRoles, "CASHIER", "%s must stay manager-only", proc)
		require.NotContains(t, p.AllowedRoles, "APOTEKER", "%s must stay manager-only", proc)
	}
}

// CanSeeCost is the redactors' only input, so its role set IS the cost policy.
// Pinned separately from the handler tests because narrowing it is the intended
// one-line way to put the boundary back — this test is what says so out loud.
func TestCanSeeCost_EveryRole(t *testing.T) {
	t.Parallel()
	for _, role := range []string{
		common.RoleOwner, common.RolePharmacist, common.RoleCashier, common.RoleApoteker,
	} {
		require.True(t, common.CanSeeCost(role), role)
	}
	require.False(t, common.CanSeeCost(""))
	require.False(t, common.CanSeeCost("SOMETHING_ELSE"))
}

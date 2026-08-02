package purchasing_test

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

// Only a STOCKED product can be bought in.
//
// A COMPOSITE is assembled from its ingredients at sale time and a SERVICE holds
// no stock at all, so receiving either would create a batch that NOTHING ever
// consumes — the sale path explodes the recipe or consumes nothing — leaving the
// goods stranded on the ledger forever while the catalog kept reporting
// buildable portions. Better to refuse the order than to accept stock that can
// never be sold.
func TestCreatePurchaseOrder_RejectsNonStockedProduct(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-KIND", "Kind supplier")

	for _, tc := range []struct {
		name string
		sku  string
		kind inventoryifacev1.ProductKind
	}{
		{"composite", "pk-menu", inventoryifacev1.ProductKind_PRODUCT_KIND_COMPOSITE},
		{"service", "pk-fee", inventoryifacev1.ProductKind_PRODUCT_KIND_SERVICE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			created, err := e.products.CreateProduct(e.ctx,
				connect.NewRequest(&inventoryifacev1.CreateProductRequest{
					Sku: tc.sku, Name: tc.sku, Unit: "porsi", UnitPrice: 20000, Kind: tc.kind,
				}))
			require.NoError(t, err)

			_, err = e.pos.CreatePurchaseOrder(e.ctx,
				connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
					SupplierId: supID,
					Items: []*purchasingifacev1.PurchaseOrderItemInput{
						{ProductId: created.Msg.Product.Id, OrderedQty: 5, UnitCostPrice: 800},
					},
				}))
			require.Error(t, err)
			require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
			var ce *connect.Error
			require.True(t, errors.As(err, &ce))
			require.Equal(t, "purchasing.product_not_stocked", ce.Message())
		})
	}
}

// The guard must not touch the ordinary path: a stocked product (including a
// pre-kind row, which reads as STOCKED) still orders fine.
func TestCreatePurchaseOrder_StockedProductStillAllowed(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-KIND-OK", "Kind supplier")
	prodID := e.seedProduct(t, "pk-ok", "Beras", 1000)

	resp, err := e.pos.CreatePurchaseOrder(e.ctx,
		connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
			SupplierId: supID,
			Items: []*purchasingifacev1.PurchaseOrderItemInput{
				{ProductId: prodID, OrderedQty: 5, UnitCostPrice: 800},
			},
		}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Order.Items, 1)
}

// UpdatePurchaseOrder replaces the whole item set, so it is a second door into
// the same table — it must be guarded too, or a DRAFT could be edited to hold a
// menu item and then received.
func TestUpdatePurchaseOrder_RejectsNonStockedProduct(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-KIND-UPD", "Kind supplier")
	prodID := e.seedProduct(t, "pk-upd", "Beras", 1000)
	po := e.createPO(t, supID, prodID, 5, 800)

	dish, err := e.products.CreateProduct(e.ctx,
		connect.NewRequest(&inventoryifacev1.CreateProductRequest{
			Sku: "pk-upd-menu", Name: "Nasi Goreng", Unit: "porsi", UnitPrice: 20000,
			Kind: inventoryifacev1.ProductKind_PRODUCT_KIND_COMPOSITE,
		}))
	require.NoError(t, err)

	_, err = e.pos.UpdatePurchaseOrder(e.ctx,
		connect.NewRequest(&purchasingifacev1.UpdatePurchaseOrderRequest{
			Id: po.Id,
			Items: []*purchasingifacev1.PurchaseOrderItemInput{
				{ProductId: dish.Msg.Product.Id, OrderedQty: 1, UnitCostPrice: 100},
			},
		}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

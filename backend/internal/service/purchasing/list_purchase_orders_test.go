package purchasing_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func TestListPurchaseOrders_HappyPath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-LPO", "List PO supplier")
	prodID := e.seedProduct(t, "lpo-sku", "LPO product", 1000)
	po := e.createPO(t, supID, prodID, 4, 700)

	resp, err := e.pos.ListPurchaseOrders(e.ctx, connect.NewRequest(&purchasingifacev1.ListPurchaseOrdersRequest{
		Limit: 100,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.Len(t, resp.Msg.Orders, 1)
	got := resp.Msg.Orders[0]
	require.Equal(t, po.Id, got.Id)
	require.Len(t, got.Items, 1)
	// enrichList denormalizes product name onto the item.
	require.Equal(t, "LPO product", got.Items[0].ProductName)
	require.Equal(t, "lpo-sku", got.Items[0].ProductSku)
}

func TestListPurchaseOrders_SupplierFilterExcludes(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supA := e.seedSupplier(t, "SUP-LPOA", "Supplier A")
	supB := e.seedSupplier(t, "SUP-LPOB", "Supplier B")
	prodID := e.seedProduct(t, "lpo2-sku", "LPO2 product", 1000)
	poA := e.createPO(t, supA, prodID, 1, 100)
	_ = e.createPO(t, supB, prodID, 1, 100)

	// Filter to supplier A only — should return exactly poA.
	resp, err := e.pos.ListPurchaseOrders(e.ctx, connect.NewRequest(&purchasingifacev1.ListPurchaseOrdersRequest{
		SupplierId: supA,
		Limit:      100,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.Len(t, resp.Msg.Orders, 1)
	require.Equal(t, poA.Id, resp.Msg.Orders[0].Id)
}

func TestListPurchaseOrders_Unauthenticated(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	_, err := e.pos.ListPurchaseOrders(context.Background(), connect.NewRequest(&purchasingifacev1.ListPurchaseOrdersRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

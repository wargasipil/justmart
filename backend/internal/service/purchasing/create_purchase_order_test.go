package purchasing_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func TestCreatePurchaseOrder_HappyPath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CPO", "PO supplier")
	prodID := e.seedProduct(t, "cpo-sku", "CPO product", 1000)

	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Note:       "first order",
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 5, UnitCostPrice: 800},
		},
	}))
	require.NoError(t, err)
	o := resp.Msg.Order
	require.NotNil(t, o)
	require.NotEmpty(t, o.Id)
	require.NotEmpty(t, o.PoNo) // assigned PO-YYYY-NNNN
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_DRAFT, o.Status)
	require.Equal(t, supID, o.SupplierId)
	require.Equal(t, "first order", o.Note)
	require.Equal(t, e.ownerID, o.CreatedBy)
	require.NotEmpty(t, o.WarehouseId) // resolved to MAIN
	require.Len(t, o.Items, 1)
	require.Equal(t, prodID, o.Items[0].ProductId)
	require.Equal(t, int32(5), o.Items[0].OrderedQty)
	require.Equal(t, int64(800), o.Items[0].UnitCostPrice)
	require.Equal(t, int64(4000), o.Subtotal) // 5 × 800
	require.Equal(t, int64(4000), o.OrderedTotal)
}

func TestCreatePurchaseOrder_NoItems(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CPO2", "PO supplier 2")

	_, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		// no items -> InvalidArgument
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreatePurchaseOrder_Unauthenticated(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	// No principal in ctx -> auth.MustPrincipal returns CodeUnauthenticated.
	_, err := e.pos.CreatePurchaseOrder(context.Background(), connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: "anything",
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: "x", OrderedQty: 1, UnitCostPrice: 1},
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

package purchasing_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func TestUpdatePurchaseOrder_HappyPath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-UPO", "Update PO supplier")
	prodID := e.seedProduct(t, "upo-sku", "UPO product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800) // subtotal 4000

	// Full-replace the items + flip note. New: 10 × 600 = 6000.
	resp, err := e.pos.UpdatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.UpdatePurchaseOrderRequest{
		Id:   po.Id,
		Note: "revised",
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 10, UnitCostPrice: 600},
		},
	}))
	require.NoError(t, err)
	o := resp.Msg.Order
	require.Equal(t, "revised", o.Note)
	require.Equal(t, int64(6000), o.Subtotal)
	require.Equal(t, int64(6000), o.OrderedTotal)
	require.Len(t, o.Items, 1)
	require.Equal(t, int32(10), o.Items[0].OrderedQty)
	require.Equal(t, int64(600), o.Items[0].UnitCostPrice)
}

func TestUpdatePurchaseOrder_NotDraftRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-UPO2", "Update PO supplier 2")
	prodID := e.seedProduct(t, "upo2-sku", "UPO2 product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800)
	e.sendPO(t, po.Id) // DRAFT -> SENT

	// Only DRAFT POs are editable.
	_, err := e.pos.UpdatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.UpdatePurchaseOrderRequest{
		Id:   po.Id,
		Note: "too late",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestUpdatePurchaseOrder_NotFound(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	_, err := e.pos.UpdatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.UpdatePurchaseOrderRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

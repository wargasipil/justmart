package purchasing_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func TestPayPurchase_HappyPath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-PP", "Pay supplier")
	prodID := e.seedProduct(t, "pp-sku", "PP product", 1000)
	po := e.createPO(t, supID, prodID, 10, 1000) // ordered_total 10000

	// Partial payment of 4000.
	resp, err := e.payments.PayPurchase(e.ctx, connect.NewRequest(&purchasingifacev1.PayPurchaseRequest{
		PurchaseOrderId: po.Id,
		Amount:          4000,
	}))
	require.NoError(t, err)
	require.Equal(t, po.Id, resp.Msg.PurchaseOrderId)
	require.Equal(t, int64(4000), resp.Msg.PaidAmount)
	require.Equal(t, int64(6000), resp.Msg.Outstanding)

	// Second payment accumulates.
	resp2, err := e.payments.PayPurchase(e.ctx, connect.NewRequest(&purchasingifacev1.PayPurchaseRequest{
		PurchaseOrderId: po.Id,
		Amount:          1000,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(5000), resp2.Msg.PaidAmount)
	require.Equal(t, int64(5000), resp2.Msg.Outstanding)
}

func TestPayPurchase_NonPositiveAmount(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-PP2", "Pay supplier 2")
	prodID := e.seedProduct(t, "pp2-sku", "PP2 product", 1000)
	po := e.createPO(t, supID, prodID, 1, 1000)

	_, err := e.payments.PayPurchase(e.ctx, connect.NewRequest(&purchasingifacev1.PayPurchaseRequest{
		PurchaseOrderId: po.Id,
		Amount:          0, // must be > 0
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestPayPurchase_VoidedRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-PP3", "Pay supplier 3")
	prodID := e.seedProduct(t, "pp3-sku", "PP3 product", 1000)
	po := e.createPO(t, supID, prodID, 1, 1000)
	_, err := e.pos.VoidPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.VoidPurchaseOrderRequest{Id: po.Id}))
	require.NoError(t, err)

	// Cannot pay a voided PO.
	_, err = e.payments.PayPurchase(e.ctx, connect.NewRequest(&purchasingifacev1.PayPurchaseRequest{
		PurchaseOrderId: po.Id,
		Amount:          500,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}
